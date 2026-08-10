//! API error envelope, mirroring the Go backend's `{code, message}` shape and
//! status-code mapping in `httpserver/server.go`.

use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::Json;
use serde::Serialize;

#[derive(Debug, Clone)]
pub struct ApiError {
    pub status: u16,
    pub message: String,
}

impl ApiError {
    pub fn bad_request(msg: impl Into<String>) -> Self {
        Self { status: 400, message: msg.into() }
    }
    pub fn unauthorized(msg: impl Into<String>) -> Self {
        Self { status: 401, message: msg.into() }
    }
    pub fn not_found(msg: impl Into<String>) -> Self {
        Self { status: 404, message: msg.into() }
    }
    pub fn conflict(msg: impl Into<String>) -> Self {
        Self { status: 409, message: msg.into() }
    }
    pub fn internal(msg: impl Into<String>) -> Self {
        Self { status: 500, message: msg.into() }
    }
    pub fn bad_gateway(msg: impl Into<String>) -> Self {
        Self { status: 502, message: msg.into() }
    }

    pub fn status(&self) -> StatusCode {
        StatusCode::from_u16(self.status).unwrap_or(StatusCode::INTERNAL_SERVER_ERROR)
    }

    /// Wire `code` name matching Go's `statusName`.
    pub fn code_name(&self) -> &'static str {
        match self.status {
            400 => "invalid_argument",
            401 => "unauthenticated",
            404 => "not_found",
            409 => "already_exists",
            502 => "upstream_unavailable",
            _ => "internal",
        }
    }
}

#[derive(Serialize)]
struct ErrorBody {
    code: &'static str,
    message: String,
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        (self.status(), Json(ErrorBody { code: self.code_name(), message: self.message })).into_response()
    }
}

/// Map a malformed JSON body / unknown enum value to the standard 400 envelope.
pub fn json_error(e: &axum::extract::rejection::JsonRejection) -> ApiError {
    ApiError::bad_request(e.body_text())
}

pub type Result<T> = std::result::Result<T, ApiError>;

impl From<sqlx::Error> for ApiError {
    fn from(e: sqlx::Error) -> Self {
        // Map the common SQLite constraint violation (duplicate email) to 409,
        // matching Go's Conflict on INSERT. Everything else is 500.
        match e {
            sqlx::Error::Database(db) if db.code().as_deref() == Some("1555") || db.is_unique_violation() => {
                ApiError::conflict("email already exists")
            }
            _ => ApiError::internal("internal error"),
        }
    }
}
