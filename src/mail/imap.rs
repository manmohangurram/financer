//! IMAP implementation of [`MailSource`], over `async-imap` + `tokio-rustls`.
//!
//! Roots are bundled at compile time, so TLS does not depend on the OS trust
//! store existing in the container.

use std::sync::Arc;

use async_imap::Session;
use async_trait::async_trait;
use futures_util::StreamExt;
use rustls::pki_types::ServerName;
use tokio::net::TcpStream;
use tokio_rustls::TlsConnector;
use tokio_rustls::client::TlsStream;
use tokio_util::compat::{Compat, TokioAsyncReadCompatExt};

use crate::error::{ApiError, Result};
use crate::mail::{MailHeader, MailSource};
use crate::repo::traits::mail::MailAccountRow;

type Stream = Compat<TlsStream<TcpStream>>;

pub struct ImapSource {
    session: tokio::sync::Mutex<Session<Stream>>,
    senders: Vec<String>,
}

impl ImapSource {
    pub async fn connect(account: &MailAccountRow, password: &str) -> Result<Self> {
        let host = account.imap_host.clone();
        let port = u16::try_from(account.imap_port).map_err(|_| {
            ApiError::bad_request(format!("imap_port {} is out of range", account.imap_port))
        })?;

        let mut roots = rustls::RootCertStore::empty();
        roots.extend(webpki_roots::TLS_SERVER_ROOTS.iter().cloned());
        let config = rustls::ClientConfig::builder()
            .with_root_certificates(roots)
            .with_no_client_auth();
        let connector = TlsConnector::from(Arc::new(config));

        let tcp = TcpStream::connect((host.as_str(), port))
            .await
            .map_err(|e| ApiError::bad_gateway(format!("imap TCP to {host}:{port} failed: {e}")))?;
        let server_name = ServerName::try_from(host.clone()).map_err(|_| {
            ApiError::bad_request(format!("imap_host {host} is not a valid DNS name"))
        })?;
        let tls = connector
            .connect(server_name, tcp)
            .await
            .map_err(|e| ApiError::bad_gateway(format!("imap TLS to {host}:{port} failed: {e}")))?
            .compat();

        let mut client = async_imap::Client::new(tls);
        client
            .read_response()
            .await
            .map_err(|e| ApiError::bad_gateway(format!("imap greeting failed: {e}")))?
            .ok_or_else(|| ApiError::bad_gateway("imap server closed before greeting"))?;
        let mut session = client
            .login(&account.username, password)
            .await
            .map_err(|(e, _)| ApiError::bad_request(format!("imap login failed: {e}")))?;
        session
            .select("INBOX")
            .await
            .map_err(|e| ApiError::bad_gateway(format!("imap SELECT INBOX failed: {e}")))?;

        Ok(Self {
            session: tokio::sync::Mutex::new(session),
            senders: account.sender_allowlist.clone(),
        })
    }

    fn since_clause(days: u32) -> String {
        let since = chrono::Utc::now() - chrono::Duration::days(i64::from(days));
        since.format("SINCE %d-%b-%Y").to_string()
    }

    fn from_clause(senders: &[String]) -> Option<String> {
        let mut parts = Vec::new();
        for sender in senders {
            let quoted = format!("\"{}\"", sender.replace('"', ""));
            parts.push(format!("FROM {quoted}"));
        }
        (!parts.is_empty()).then(|| format!("({})", parts.join(" OR ")))
    }
}

#[async_trait]
impl MailSource for ImapSource {
    async fn headers(&self, _senders: &[String], since_days: u32) -> Result<Vec<MailHeader>> {
        let Some(from) = Self::from_clause(&self.senders) else {
            return Ok(Vec::new());
        };
        let query = format!("{from} {}", Self::since_clause(since_days));

        let mut session = self.session.lock().await;
        let uids = session
            .uid_search(&query)
            .await
            .map_err(|e| ApiError::bad_gateway(format!("imap SEARCH failed: {e}")))?;
        if uids.is_empty() {
            return Ok(Vec::new());
        }
        let set = uids
            .iter()
            .map(u32::to_string)
            .collect::<Vec<_>>()
            .join(",");

        let mut fetches = session
            .uid_fetch(
                &set,
                "(UID BODY.PEEK[HEADER.FIELDS (MESSAGE-ID FROM SUBJECT DATE)])",
            )
            .await
            .map_err(|e| ApiError::bad_gateway(format!("imap FETCH headers failed: {e}")))?;

        let mut out = Vec::new();
        while let Some(item) = fetches.next().await {
            let fetch =
                item.map_err(|e| ApiError::bad_gateway(format!("imap FETCH headers failed: {e}")))?;
            let (Some(uid), Some(raw)) = (fetch.uid, fetch.header()) else {
                continue;
            };
            // No Message-ID means no idempotency key, so the message cannot be
            // deduped; skipping beats ingesting it repeatedly.
            let Some(message_id) = parse_message_id(raw) else {
                continue;
            };
            out.push(MailHeader {
                uid,
                message_id,
                from_addr: parse_header_value(raw, "From").unwrap_or_default(),
                subject: parse_header_value(raw, "Subject").unwrap_or_default(),
                received_at: parse_header_value(raw, "Date"),
            });
        }
        Ok(out)
    }

    async fn body(&self, uid: u32) -> Result<String> {
        let mut session = self.session.lock().await;
        let mut fetches = session
            .uid_fetch(uid.to_string(), "(BODY.PEEK[])")
            .await
            .map_err(|e| ApiError::bad_gateway(format!("imap FETCH body failed: {e}")))?;
        while let Some(item) = fetches.next().await {
            let fetch =
                item.map_err(|e| ApiError::bad_gateway(format!("imap FETCH body failed: {e}")))?;
            if let Some(raw) = fetch.body() {
                return Ok(String::from_utf8_lossy(raw).into_owned());
            }
        }
        Err(ApiError::bad_gateway("imap returned no body"))
    }
}

pub fn parse_message_id(raw: &[u8]) -> Option<String> {
    let text = String::from_utf8_lossy(raw);
    for line in text.lines() {
        if let Some(rest) = strip_prefix_ci(line, "message-id:") {
            let id = rest.trim();
            if !id.is_empty() {
                return Some(id.to_string());
            }
        }
    }
    None
}

pub fn parse_header_value(raw: &[u8], name: &str) -> Option<String> {
    let text = String::from_utf8_lossy(raw);
    let needle = format!("{name}:");
    for line in text.lines() {
        if let Some(rest) = strip_prefix_ci(line, &needle) {
            return Some(rest.trim().to_string());
        }
    }
    None
}

fn strip_prefix_ci<'a>(line: &'a str, prefix: &str) -> Option<&'a str> {
    let head = line.get(..prefix.len())?;
    head.eq_ignore_ascii_case(prefix)
        .then(|| &line[prefix.len()..])
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_the_headers_the_poll_needs() {
        let raw = b"From: alerts@hdfcbank.net\r\nDate: Tue, 15 Sep 2026 10:00:00 +0530\r\nMessage-ID: <abc.123@hdfcbank.net>\r\nSubject: Debited\r\n\r\nbody";
        assert_eq!(
            parse_message_id(raw).as_deref(),
            Some("<abc.123@hdfcbank.net>")
        );
        assert_eq!(
            parse_header_value(raw, "From").as_deref(),
            Some("alerts@hdfcbank.net")
        );
        assert_eq!(
            parse_header_value(raw, "Subject").as_deref(),
            Some("Debited")
        );
        assert_eq!(
            parse_header_value(raw, "Date").as_deref(),
            Some("Tue, 15 Sep 2026 10:00:00 +0530")
        );
    }

    #[test]
    fn header_lookup_is_case_insensitive_and_absent_safe() {
        let raw = b"message-id: <x@y>\r\n";
        assert_eq!(parse_message_id(raw).as_deref(), Some("<x@y>"));
        assert_eq!(parse_header_value(raw, "Subject"), None);
        assert_eq!(parse_message_id(b""), None);
    }

    #[test]
    fn from_clause_or_joins_the_allowlist_and_ignores_quotes() {
        assert_eq!(ImapSource::from_clause(&[]), None);
        let one = ImapSource::from_clause(&["a@b.c".into()]).unwrap();
        assert_eq!(one, "(FROM \"a@b.c\")");
        let two = ImapSource::from_clause(&["a@b.c".into(), "d@e.f".into()]).unwrap();
        assert_eq!(two, "(FROM \"a@b.c\" OR FROM \"d@e.f\")");
        let injected = ImapSource::from_clause(&["a\" OR ALL".into()]).unwrap();
        assert_eq!(injected, "(FROM \"a OR ALL\")");
    }
}
