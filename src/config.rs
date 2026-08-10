//! Runtime configuration from environment variables (FINANCER_*), mirroring the
//! Go backend's env contract.

use std::path::PathBuf;

pub struct Config {
    pub addr: String,
    pub data_dir: PathBuf,
    pub db_path: PathBuf,
    pub jwt_secret: String,
    #[allow(dead_code)]
    pub rust_routes: Vec<String>,
    pub static_dir: PathBuf,
    pub domain_url: String,
}

impl Config {
    pub fn from_env() -> Self {
        let data_dir = env("FINANCER_DATA_DIR").map_or_else(|| PathBuf::from("data"), PathBuf::from);

        let db_path = env("FINANCER_DB_PATH").map_or_else(
            || data_dir.join("db").join("financer.db"),
            PathBuf::from,
        );

        let rust_routes = env("FINANCER_RUST_ROUTES")
            .map(|v| v.split(',').map(|s| s.trim().to_string()).filter(|s| !s.is_empty()).collect())
            .unwrap_or_default();

        // An empty host (":8080") fails getaddrinfo on musl; bind all interfaces.
        let raw_addr = env("FINANCER_ADDR").unwrap_or_else(|| "0.0.0.0:8080".to_string());
        let addr = if raw_addr.starts_with(':') { format!("0.0.0.0{raw_addr}") } else { raw_addr };

        Config {
            addr,
            data_dir,
            db_path,
            jwt_secret: env("FINANCER_JWT_SECRET")
                .unwrap_or_else(|| "dev-secret-change-in-production".to_string()),
            rust_routes,
            static_dir: env("FINANCER_STATIC_DIR")
                .map_or_else(|| PathBuf::from("frontend/dist"), PathBuf::from),
            domain_url: env("FINANCER_DOMAIN_URL").unwrap_or_default(),
        }
    }
}

fn env(key: &str) -> Option<String> {
    std::env::var(key).ok().filter(|v| !v.is_empty())
}
