//! JWT HS256 access/refresh tokens, mirroring Go's `auth/auth.go`.

use chrono::Utc;
use jsonwebtoken::{DecodingKey, EncodingKey, Header, Validation, decode, encode};
use serde::{Deserialize, Serialize};

use crate::error::{ApiError, Result};

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Claims {
    #[serde(rename = "user_id")]
    pub user_id: String,
    pub email: String,
    #[serde(rename = "ver")]
    pub token_version: i64,
    /// "access" or "refresh" — which token kind this is.
    #[serde(rename = "typ")]
    pub typ: String,
    /// registered claims
    pub exp: u64,
    pub iat: u64,
    pub iss: String,
    pub sub: String,
}

#[derive(Clone)]
pub struct Jwt {
    secret: String,
}

impl Jwt {
    pub fn new(secret: String) -> Self {
        Self { secret }
    }

    pub fn access_token(&self, user_id: &str, email: &str, ver: i64) -> Result<String> {
        let now = Utc::now();
        let claims = Claims {
            user_id: user_id.to_string(),
            email: email.to_string(),
            token_version: ver,
            typ: "access".to_string(),
            exp: u64::try_from((now + chrono::Duration::hours(24)).timestamp()).unwrap_or(u64::MAX),
            iat: u64::try_from(now.timestamp()).unwrap_or_default(),
            iss: "financer".to_string(),
            sub: user_id.to_string(),
        };
        encode(
            &Header::default(),
            &claims,
            &EncodingKey::from_secret(self.secret.as_bytes()),
        )
        .map_err(|_| ApiError::internal("failed to generate token"))
    }

    pub fn refresh_token(&self, user_id: &str, ver: i64) -> Result<String> {
        let now = Utc::now();
        let claims = Claims {
            user_id: user_id.to_string(),
            email: String::new(),
            token_version: ver,
            typ: "refresh".to_string(),
            exp: u64::try_from((now + chrono::Duration::days(30)).timestamp()).unwrap_or(u64::MAX),
            iat: u64::try_from(now.timestamp()).unwrap_or_default(),
            iss: "financer".to_string(),
            sub: user_id.to_string(),
        };
        encode(
            &Header::default(),
            &claims,
            &EncodingKey::from_secret(self.secret.as_bytes()),
        )
        .map_err(|_| ApiError::internal("failed to generate token"))
    }

    pub fn validate(&self, token: &str) -> Result<Claims> {
        let mut validation = Validation::new(jsonwebtoken::Algorithm::HS256);
        validation.set_issuer(&["financer"]);
        decode::<Claims>(
            token,
            &DecodingKey::from_secret(self.secret.as_bytes()),
            &validation,
        )
        .map(|d| d.claims)
        .map_err(|_| ApiError::unauthorized("invalid token"))
    }
}
