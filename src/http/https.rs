//! Refuses to accept credentials over an unencrypted connection.
//!
//! The API authenticates with a bearer token, so plain HTTP exposes the whole
//! session, not just one field. Credential-bearing endpoints therefore require
//! TLS, with loopback exempted so local development needs no certificate.

use std::net::IpAddr;

use std::sync::atomic::{AtomicBool, Ordering};

use axum::extract::ConnectInfo;
use axum::http::Request;
use axum::middleware::Next;
use axum::response::{IntoResponse, Response};

use crate::error::ApiError;

/// Set once at boot from `FINANCER_ALLOW_INSECURE_HTTP`. A global because the
/// guard runs as `from_fn` middleware, before the router's state is attached.
static ALLOW_INSECURE: AtomicBool = AtomicBool::new(false);

pub fn configure(allow_insecure: bool) {
    ALLOW_INSECURE.store(allow_insecure, Ordering::Relaxed);
}

pub async fn require_https(req: Request<axum::body::Body>, next: Next) -> Response {
    let forwarded = req
        .headers()
        .get("x-forwarded-proto")
        .and_then(|v| v.to_str().ok());
    let peer = req
        .extensions()
        .get::<ConnectInfo<std::net::SocketAddr>>()
        .map(|c| c.0.ip());
    if is_secure(
        forwarded,
        req.uri().scheme_str(),
        peer,
        ALLOW_INSECURE.load(Ordering::Relaxed),
    ) {
        next.run(req).await
    } else {
        ApiError::forbidden("this endpoint requires HTTPS").into_response()
    }
}

/// A reverse proxy terminates TLS, so the scheme it reports is authoritative
/// when present; otherwise the URI scheme, and finally loopback (dev).
fn is_secure(
    forwarded: Option<&str>,
    scheme: Option<&str>,
    peer: Option<IpAddr>,
    allow_insecure: bool,
) -> bool {
    if allow_insecure {
        return true;
    }
    if scheme == Some("https") {
        return true;
    }
    if forwarded.is_some_and(|p| p.eq_ignore_ascii_case("https")) {
        return true;
    }
    peer.is_some_and(|ip| ip.is_loopback())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::Ipv6Addr;

    fn ip(s: &str) -> IpAddr {
        s.parse().unwrap()
    }

    #[test]
    fn https_scheme_is_enough() {
        assert!(is_secure(None, Some("https"), None, false));
    }

    #[test]
    fn a_proxy_reporting_https_is_trusted() {
        assert!(is_secure(Some("https"), Some("http"), None, false));
        assert!(is_secure(Some("HTTPS"), Some("http"), None, false));
    }

    #[test]
    fn loopback_is_exempt_for_development() {
        assert!(is_secure(None, Some("http"), Some(ip("127.0.0.1")), false));
        assert!(is_secure(
            None,
            Some("http"),
            Some(IpAddr::V6(Ipv6Addr::LOCALHOST)),
            false
        ));
    }

    #[test]
    fn the_override_accepts_plain_http() {
        assert!(is_secure(
            None,
            Some("http"),
            Some(ip("192.168.1.20")),
            true
        ));
    }

    #[test]
    fn a_lan_client_over_http_is_refused() {
        assert!(!is_secure(
            None,
            Some("http"),
            Some(ip("192.168.1.20")),
            false
        ));
        assert!(!is_secure(
            Some("http"),
            Some("http"),
            Some(ip("10.0.0.5")),
            false
        ));
        assert!(!is_secure(None, None, None, false));
    }
}
