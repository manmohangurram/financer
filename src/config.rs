//! Runtime configuration from `FINANCER_*` environment variables.
//!
//! There is no config file: every setting has a built-in default and can be
//! overridden by an env var (see the README table). Storage is UTC and the
//! timezone defaults to `Asia/Kolkata`.

use std::path::PathBuf;

#[derive(Debug, Clone, Default)]
pub struct Config {
    pub server: Server,
    pub storage: Storage,
    pub yahoo: YahooConfigSection,
}

#[derive(Debug, Clone)]
pub struct Server {
    pub addr: String,
    pub data_dir: PathBuf,
    pub static_dir: PathBuf,
    pub domain_url: String,
    pub jwt_secret: String,
    /// IANA timezone used for all date/time interpretation (e.g. "Asia/Kolkata").
    pub timezone: String,
}

impl Default for Server {
    fn default() -> Self {
        Self {
            addr: "0.0.0.0:8080".into(),
            data_dir: "/data".into(),
            static_dir: PathBuf::from("frontend/dist"),
            domain_url: String::new(),
            jwt_secret: String::new(),
            timezone: "Asia/Kolkata".into(),
        }
    }
}

#[derive(Debug, Clone)]
pub struct Storage {
    /// "sqlite" | "surreal"
    pub database: String,
    pub sqlite: Sqlite,
    pub surreal: Surreal,
}

impl Default for Storage {
    fn default() -> Self {
        Self {
            database: "sqlite".into(),
            sqlite: Sqlite::default(),
            surreal: Surreal::default(),
        }
    }
}

/// Advanced SQLite tuning (maps to sqlx `SqliteConnectOptions`).
#[derive(Debug, Clone)]
pub struct Sqlite {
    pub path: String,
    /// "delete" | "truncate" | "persist" | "memory" | "wal" | "off"
    pub journal_mode: String,
    /// "off" | "normal" | "full" | "extra"
    pub synchronous: String,
    pub busy_timeout_ms: u64,
    pub foreign_keys: bool,
    /// Bytes; SQLite page size. 0/unset = SQLite default (4096).
    pub page_size: u32,
    pub write_pool_size: u32,
    pub read_pool_size: u32,
}

impl Default for Sqlite {
    fn default() -> Self {
        Self {
            path: "/data/financer.db".into(),
            journal_mode: "wal".into(),
            synchronous: "normal".into(),
            busy_timeout_ms: 5000,
            foreign_keys: true,
            page_size: 0,
            write_pool_size: 1,
            read_pool_size: 5,
        }
    }
}

#[derive(Debug, Clone)]
pub struct Surreal {
    pub url: String,
    pub user: String,
    pub pass: String,
    pub ns: String,
    pub db: String,
}

impl Default for Surreal {
    fn default() -> Self {
        Self {
            url: "127.0.0.1:8000".into(),
            user: "root".into(),
            pass: "root".into(),
            ns: "financer".into(),
            db: "financer".into(),
        }
    }
}

/// The Yahoo Finance client config.
#[derive(Debug, Clone)]
pub struct YahooConfigSection {
    pub bases: Vec<String>,
    pub chart: String,
    pub search: String,
}

impl Default for YahooConfigSection {
    fn default() -> Self {
        Self {
            bases: vec![
                "https://query1.finance.yahoo.com".into(),
                "https://query2.finance.yahoo.com".into(),
            ],
            chart: "/v8/finance/chart/{symbol}?interval={interval}&period1={period1}&period2={period2}&includePrePost=true&events=div%7Csplit%7Cearn&lang=en-US&region=US&source=cosaic".into(),
            search: "/v1/finance/search?q={query}&quotesCount=8&newsCount=0".into(),
        }
    }
}

/// Storage database choice, derived from `Storage.database`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Database {
    Surreal,
    Sqlite,
}

impl Config {
    /// Build the config from `FINANCER_*` env vars, starting from defaults.
    /// Blank/unset vars leave the default in place. Fails if
    /// `FINANCER_JWT_SECRET` is missing (there is no persistent secret store).
    pub fn from_env() -> anyhow::Result<Self> {
        let mut cfg = Config::default();
        cfg.apply_overrides_from(|k| std::env::var(k).ok().filter(|v| !v.is_empty()));
        if cfg.server.jwt_secret.is_empty() {
            anyhow::bail!(
                "FINANCER_JWT_SECRET is required — set it to a stable secret (e.g. `openssl rand -hex 32`)"
            );
        }
        Ok(cfg)
    }

    fn apply_overrides_from(&mut self, get: impl Fn(&str) -> Option<String>) {
        if let Some(v) = get("FINANCER_ADDR") {
            self.server.addr = v;
        }
        if let Some(v) = get("FINANCER_DATA_DIR") {
            self.server.data_dir = v.into();
        }
        if let Some(v) = get("FINANCER_DOMAIN_URL") {
            self.server.domain_url = v;
        }
        if let Some(v) = get("FINANCER_JWT_SECRET") {
            self.server.jwt_secret = v;
        }
        if let Some(v) = get("FINANCER_TIMEZONE") {
            self.server.timezone = v;
        }
        if let Some(v) = get("FINANCER_DATABASE") {
            self.storage.database = v;
        }
        if let Some(v) = get("FINANCER_SQLITE_PATH") {
            self.storage.sqlite.path = v;
        }
        if let Some(v) = get("FINANCER_SURREAL_URL") {
            self.storage.surreal.url = v;
        }
        if let Some(v) = get("FINANCER_SURREAL_USER") {
            self.storage.surreal.user = v;
        }
        if let Some(v) = get("FINANCER_SURREAL_PASS") {
            self.storage.surreal.pass = v;
        }
        if let Some(v) = get("FINANCER_SURREAL_NS") {
            self.storage.surreal.ns = v;
        }
        if let Some(v) = get("FINANCER_SURREAL_DB") {
            self.storage.surreal.db = v;
        }
    }

    /// Choose the database from `[storage] database` value.
    pub fn database(&self) -> Database {
        if self.storage.database.eq_ignore_ascii_case("surreal") {
            Database::Surreal
        } else {
            Database::Sqlite
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn map<'a>(pairs: &'a [(&'a str, &'a str)]) -> impl Fn(&str) -> Option<String> + 'a {
        move |k| {
            pairs
                .iter()
                .find(|(key, _)| *key == k)
                .map(|(_, v)| (*v).to_string())
        }
    }

    #[test]
    fn env_vars_win_over_defaults() {
        let mut cfg = Config::default();
        cfg.apply_overrides_from(map(&[
            ("FINANCER_ADDR", "0.0.0.0:9000"),
            ("FINANCER_JWT_SECRET", "secret"),
            ("FINANCER_TIMEZONE", "Asia/Kolkata"),
            ("FINANCER_DATABASE", "surreal"),
            ("FINANCER_SURREAL_URL", "surrealdb:8000"),
            ("FINANCER_SURREAL_PASS", "pw"),
        ]));
        assert_eq!(cfg.server.addr, "0.0.0.0:9000");
        assert_eq!(cfg.server.jwt_secret, "secret");
        assert_eq!(cfg.server.timezone, "Asia/Kolkata");
        assert_eq!(cfg.storage.database, "surreal");
        assert_eq!(cfg.storage.surreal.url, "surrealdb:8000");
        assert_eq!(cfg.storage.surreal.pass, "pw");
        // Unset vars keep their defaults.
        assert_eq!(cfg.storage.surreal.user, "root");
        assert_eq!(cfg.storage.sqlite.path, "/data/financer.db");
        assert_eq!(cfg.database(), Database::Surreal);
    }

    #[test]
    fn blank_env_values_are_ignored() {
        let mut cfg = Config::default();
        cfg.apply_overrides_from(|_| None); // nothing set
        assert_eq!(cfg.server.addr, "0.0.0.0:8080");
        assert_eq!(cfg.storage.database, "sqlite");
    }
}
