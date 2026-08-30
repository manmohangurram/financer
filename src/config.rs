//! Runtime configuration from environment variables.
//!
//! The storage backend is inferred from which env vars are set:
//! - `SQLITE_DB_PATH` set → `SQLite`
//! - otherwise (`SURREAL_DB_*`) → `SurrealDB` (default)

use std::path::PathBuf;

pub enum Backend {
    Surreal,
    Sqlite,
}

pub struct Config {
    pub backend: Backend,
    pub addr: String,
    pub data_dir: PathBuf,
    pub jwt_secret: String,
    pub static_dir: PathBuf,
    pub domain_url: String,
    pub sqlite_path: String,
    pub surreal_url: String,
    pub surreal_user: String,
    pub surreal_pass: String,
    pub surreal_ns: String,
    pub surreal_db: String,
}

impl Config {
    pub fn from_env() -> Self {
        let data_dir = env("FINANCER_DATA_DIR").map_or_else(|| PathBuf::from("data"), PathBuf::from);

        // An empty host (":8080") fails getaddrinfo on musl; bind all interfaces.
        let raw_addr = env("FINANCER_ADDR").unwrap_or_else(|| "0.0.0.0:8080".to_string());
        let addr = if raw_addr.starts_with(':') { format!("0.0.0.0{raw_addr}") } else { raw_addr };

        // Bare env names per the chosen scheme. SQLite is selected by presence
        // of `SQLITE_DB_PATH`; SurrealDB is the default when it's absent.
        let sqlite_path = env("SQLITE_DB_PATH").unwrap_or_default();
        let backend = if sqlite_path.is_empty() { Backend::Surreal } else { Backend::Sqlite };

        Config {
            backend,
            addr,
            data_dir,
            jwt_secret: env("FINANCER_JWT_SECRET")
                .unwrap_or_else(|| "dev-secret-change-in-production".to_string()),
            static_dir: env("FINANCER_STATIC_DIR")
                .map_or_else(|| PathBuf::from("frontend/dist"), PathBuf::from),
            domain_url: env("FINANCER_DOMAIN_URL").unwrap_or_default(),
            sqlite_path,
            surreal_url: env("SURREAL_DB_URL").unwrap_or_else(|| "127.0.0.1:8000".into()),
            surreal_user: env("SURREAL_DB_USER").unwrap_or_else(|| "root".into()),
            surreal_pass: env("SURREAL_DB_PASS").unwrap_or_else(|| "root".into()),
            surreal_ns: env("SURREAL_DB_NS").unwrap_or_else(|| "financer".into()),
            surreal_db: env("SURREAL_DB_DB").unwrap_or_else(|| "financer".into()),
        }
    }
}

fn env(key: &str) -> Option<String> {
    std::env::var(key).ok().filter(|v| !v.is_empty())
}
