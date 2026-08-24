//! Runtime configuration from environment variables (FINANCER_*).

use std::path::PathBuf;

pub struct Config {
    pub addr: String,
    pub data_dir: PathBuf,
    pub jwt_secret: String,
    pub static_dir: PathBuf,
    pub domain_url: String,
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

        Config {
            addr,
            data_dir,
            jwt_secret: env("FINANCER_JWT_SECRET")
                .unwrap_or_else(|| "dev-secret-change-in-production".to_string()),
            static_dir: env("FINANCER_STATIC_DIR")
                .map_or_else(|| PathBuf::from("frontend/dist"), PathBuf::from),
            domain_url: env("FINANCER_DOMAIN_URL").unwrap_or_default(),
            surreal_url: env("FINANCER_SURREAL_URL").unwrap_or_else(|| "127.0.0.1:8000".into()),
            surreal_user: env("FINANCER_SURREAL_USER").unwrap_or_else(|| "root".into()),
            surreal_pass: env("FINANCER_SURREAL_PASS").unwrap_or_else(|| "root".into()),
            surreal_ns: env("FINANCER_SURREAL_NS").unwrap_or_else(|| "financer".into()),
            surreal_db: env("FINANCER_SURREAL_DB").unwrap_or_else(|| "financer".into()),
        }
    }
}

fn env(key: &str) -> Option<String> {
    std::env::var(key).ok().filter(|v| !v.is_empty())
}
