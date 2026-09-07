//! Runtime configuration from `config.toml`.
//!
//! The file is read at `/data/config/config.toml` (the `/data` volume). If it
//! is missing, a default is written on first run: SQLite storage and a
//! freshly generated 64-char JWT secret.

use std::fs;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

/// Default config path: `<data_dir>/config/config.toml`, where `data_dir`
/// defaults to `/data` (the user-defined volume).
pub fn default_config_path() -> PathBuf {
    PathBuf::from("/data/config/config.toml")
}

/// Find the config path to read: honour `FINANCER_CONFIG` if set (tests/dev),
/// else `{data_dir}/config/config.toml` with `/data` default.
fn config_path_for_read() -> PathBuf {
    std::env::var("FINANCER_CONFIG")
        .ok()
        .filter(|v| !v.is_empty())
        .map_or_else(default_config_path, PathBuf::from)
}

/// Generate a 64-character random alphanumeric secret.
pub fn generate_secret() -> String {
    use rand::Rng;
    const CHARS: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
    let mut rng = rand::thread_rng();
    (0..64).map(|_| CHARS[rng.gen_range(0..CHARS.len())] as char).collect()
}

#[derive(Debug, Deserialize, Serialize, Clone, Default)]
#[serde(default)]
pub struct Config {
    pub server: Server,
    pub storage: Storage,
    pub yahoo: YahooConfigSection,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(default)]
pub struct Server {
    pub addr: String,
    pub data_dir: PathBuf,
    pub static_dir: PathBuf,
    pub domain_url: String,
    pub jwt_secret: String,
}

impl Default for Server {
    fn default() -> Self {
        Self {
            addr: "0.0.0.0:8080".into(),
            data_dir: "/data".into(),
            static_dir: PathBuf::from("frontend/dist"),
            domain_url: String::new(),
            jwt_secret: generate_secret(),
        }
    }
}

#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(default)]
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
#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(default)]
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

#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(default)]
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

/// The Yahoo Finance client config (from `[yahoo]` in config.toml).
#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(default)]
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
#[derive(Clone, Copy, PartialEq, Eq)]
pub enum Database {
    Surreal,
    Sqlite,
}

impl Config {
    /// Load config from `config.toml`, auto-generating a default at
    /// `{data_dir}/config/config.toml` if none exists.
    pub fn load() -> anyhow::Result<Self> {
        let path = config_path_for_read();
        let cfg = if path.exists() {
            let raw = fs::read_to_string(&path)?;
            let mut cfg: Config = toml::from_str(&raw)?;
            // If the example was copied with an empty secret, generate one.
            if cfg.server.jwt_secret.is_empty() {
                cfg.server.jwt_secret = generate_secret();
            }
            cfg
        } else {
            let default = Config::default();
            default.write_default(&path)?;
            default
        };
        Ok(cfg)
    }

    /// Save the default config to `path`, creating parent dirs.
    fn write_default(&self, path: &Path) -> anyhow::Result<()> {
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }
        let toml_str = toml::to_string_pretty(self)?;
        fs::write(path, toml_str)?;
        Ok(())
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
