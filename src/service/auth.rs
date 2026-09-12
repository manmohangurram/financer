//! Auth service — signup / login / refresh, mirroring Go's `services/auth.go`.

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use utoipa::ToSchema;

use crate::auth::Jwt;
use crate::error::{ApiError, Result};
use crate::repo::traits::UserRepo;

#[derive(Debug, Serialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct AuthResponse {
    pub access_token: String,
    pub refresh_token: String,
    pub user_id: String,
    pub email: String,
    pub name: String,
}

#[derive(Debug, Deserialize, ToSchema)]
pub struct SignupRequest {
    pub email: String,
    pub password: String,
    pub name: String,
}

#[derive(Debug, Deserialize, ToSchema)]
pub struct LoginRequest {
    pub email: String,
    pub password: String,
}

#[derive(Debug, Deserialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct RefreshTokenRequest {
    pub refresh_token: String,
}

#[derive(Clone)]
pub struct AuthService {
    repo: Arc<dyn UserRepo>,
    jwt: Jwt,
}

impl AuthService {
    pub fn new(repo: Arc<dyn UserRepo>, jwt: Jwt) -> Self {
        Self { repo, jwt }
    }

    pub async fn signup(&self, req: SignupRequest) -> Result<AuthResponse> {
        if req.email.is_empty() || req.password.is_empty() || req.name.is_empty() {
            return Err(ApiError::bad_request(
                "email, password, and name are required",
            ));
        }
        if req.password.len() < 6 {
            return Err(ApiError::bad_request(
                "password must be at least 6 characters",
            ));
        }

        let req_password = req.password.clone();
        let hash =
            tokio::task::spawn_blocking(move || bcrypt::hash(&req_password, bcrypt::DEFAULT_COST))
                .await
                .map_err(|_| ApiError::internal("internal error"))?
                .map_err(|_| ApiError::internal("internal error"))?;

        // Go's Signup returns Conflict("email already exists") on the unique
        // email constraint violation (mapped from sqlx::Error via From).
        let id = self.repo.create(&req.email, &hash, &req.name).await?;

        self.issue_tokens(&id, &req.email, &req.name).await
    }

    pub async fn login(&self, req: LoginRequest) -> Result<AuthResponse> {
        if req.email.is_empty() || req.password.is_empty() {
            return Err(ApiError::bad_request("email and password are required"));
        }

        let user = self
            .repo
            .by_email(&req.email)
            .await?
            .ok_or_else(|| ApiError::unauthorized("invalid credentials"))?;
        let stored = user.password_hash.unwrap_or_default();
        let password = req.password.clone();
        let ok = tokio::task::spawn_blocking(move || bcrypt::verify(&password, &stored))
            .await
            .map_err(|_| ApiError::internal("internal error"))?
            .unwrap_or(false);
        if !ok {
            return Err(ApiError::unauthorized("invalid credentials"));
        }

        self.issue_tokens(&user.id, &user.email, user.name.as_deref().unwrap_or(""))
            .await
    }

    pub async fn refresh_token(&self, req: RefreshTokenRequest) -> Result<AuthResponse> {
        if req.refresh_token.is_empty() {
            return Err(ApiError::bad_request("refresh_token is required"));
        }
        let claims = self.jwt.validate(&req.refresh_token)?;
        if claims.typ != "refresh" {
            return Err(ApiError::unauthorized("invalid refresh token"));
        }

        let user = self
            .repo
            .by_id(&claims.user_id)
            .await?
            .ok_or_else(|| ApiError::unauthorized("user not found"))?;
        if claims.token_version != user.token_version {
            return Err(ApiError::unauthorized("session has been revoked"));
        }

        self.issue_tokens(&user.id, &user.email, user.name.as_deref().unwrap_or(""))
            .await
    }

    async fn issue_tokens(&self, user_id: &str, email: &str, name: &str) -> Result<AuthResponse> {
        let user = self
            .repo
            .by_id(user_id)
            .await?
            .ok_or_else(|| ApiError::internal("failed to read user"))?;
        let ver = user.token_version;
        let access = self.jwt.access_token(user_id, email, ver)?;
        let refresh = self.jwt.refresh_token(user_id, ver)?;
        Ok(AuthResponse {
            access_token: access,
            refresh_token: refresh,
            user_id: user_id.to_string(),
            email: email.to_string(),
            name: name.to_string(),
        })
    }
}
