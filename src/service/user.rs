//! User service — profile reads/updates, password change, logout-all, avatar,
//! mirroring Go's `services/user.go`.

use serde::Serialize;
use std::path::PathBuf;
use std::sync::Arc;
use utoipa::ToSchema;

use crate::auth::Jwt;
use crate::error::{ApiError, Result};
use crate::repo::traits::UserRepo;
use crate::service::auth::AuthResponse;

#[derive(Clone)]
pub struct UserService {
    repo: Arc<dyn UserRepo>,
    jwt: Jwt,
    avatar_dir: PathBuf,
}

/// Wire profile (serde covers Go's `wireProfile`).
#[derive(Debug, Serialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct ProfileResponse {
    pub user_id: String,
    pub name: String,
    pub email: String,
    pub avatar_url: String,
}

const MAX_AVATAR_BYTES: usize = 5 << 20;

impl UserService {
    pub fn new(repo: Arc<dyn UserRepo>, jwt: Jwt, avatar_dir: PathBuf) -> Self {
        Self {
            repo,
            jwt,
            avatar_dir,
        }
    }

    pub async fn profile(&self, user_id: &str) -> Result<ProfileResponse> {
        let user = self
            .repo
            .by_id(user_id)
            .await?
            .ok_or_else(|| ApiError::not_found("user not found"))?;
        Ok(ProfileResponse {
            user_id: user.id,
            name: user.name.unwrap_or_default(),
            email: user.email,
            avatar_url: user.avatar_url.unwrap_or_default(),
        })
    }

    pub async fn update_profile(
        &self,
        user_id: &str,
        name: &str,
        email: &str,
        avatar_url: &str,
    ) -> Result<ProfileResponse> {
        if name.is_empty() && email.is_empty() && avatar_url.is_empty() {
            return Err(ApiError::bad_request("nothing to update"));
        }
        self.repo
            .update_profile(user_id, name, email, avatar_url)
            .await
            .map_err(|_| ApiError::conflict("email already in use"))?;
        self.profile(user_id).await
    }

    /// Verify current password, update hash, bump `token_version`, re-issue tokens.
    pub async fn change_password(
        &self,
        user_id: &str,
        current_password: &str,
        new_password: &str,
    ) -> Result<AuthResponse> {
        if new_password.is_empty() {
            return Err(ApiError::bad_request("new password is required"));
        }
        if new_password.len() < 6 {
            return Err(ApiError::bad_request(
                "new password must be at least 6 characters",
            ));
        }

        let user = self
            .repo
            .by_id(user_id)
            .await?
            .ok_or_else(|| ApiError::not_found("user not found"))?;
        let stored = user.password_hash.clone().unwrap_or_default();
        let current = current_password.to_string();
        let ok = tokio::task::spawn_blocking(move || bcrypt::verify(&current, &stored))
            .await
            .map_err(|_| ApiError::internal("internal error"))?
            .unwrap_or(false);
        if !ok {
            return Err(ApiError::bad_request("current password is incorrect"));
        }

        let new = new_password.to_string();
        let hash = tokio::task::spawn_blocking(move || bcrypt::hash(&new, bcrypt::DEFAULT_COST))
            .await
            .map_err(|_| ApiError::internal("internal error"))?
            .map_err(|_| ApiError::internal("internal error"))?;
        let ver = self
            .repo
            .change_password(user_id, &hash)
            .await
            .map_err(|_| ApiError::internal("failed to update password"))?;

        let access = self.jwt.access_token(user_id, &user.email, ver)?;
        let refresh = self.jwt.refresh_token(user_id, ver)?;
        Ok(AuthResponse {
            access_token: access,
            refresh_token: refresh,
            user_id: user_id.to_string(),
            email: user.email,
            name: user.name.unwrap_or_default(),
        })
    }

    /// Bump `token_version` so every previously issued refresh token is rejected.
    pub async fn logout_all(&self, user_id: &str) -> Result<()> {
        self.repo
            .logout_all(user_id)
            .await
            .map_err(|_| ApiError::internal("failed to log out all sessions"))
    }

    /// Store an avatar image (sniffed content type, 5MB cap) and update `avatar_url`.
    pub async fn save_avatar(&self, user_id: &str, data: &[u8]) -> Result<ProfileResponse> {
        if data.is_empty() {
            return Err(ApiError::bad_request("failed to read avatar"));
        }
        if data.len() > MAX_AVATAR_BYTES {
            return Err(ApiError::bad_request("avatar must be 5MB or smaller"));
        }
        let Some(ext) = infer_avatar_type(data) else {
            return Err(ApiError::bad_request(
                "avatar must be a png, jpeg, or webp image",
            ));
        };

        std::fs::create_dir_all(&self.avatar_dir)
            .map_err(|_| ApiError::internal("failed to save avatar"))?;
        let filename = format!("{user_id}{ext}");
        let path = self.avatar_dir.join(&filename);
        std::fs::write(&path, data).map_err(|_| ApiError::internal("failed to save avatar"))?;
        let avatar_url = format!("/avatars/{filename}");

        self.repo
            .update_profile(user_id, "", "", &avatar_url)
            .await
            .map_err(|_| ApiError::internal("failed to update avatar"))?;
        self.profile(user_id).await
    }
}

/// Sniff an image content type → extension, mirroring Go's `DetectContentType` map.
fn infer_avatar_type(data: &[u8]) -> Option<&'static str> {
    if data.starts_with(b"\x89PNG\r\n\x1a\n") {
        return Some(".png");
    }
    if data.starts_with(b"\xff\xd8\xff") {
        return Some(".jpg");
    }
    if data.len() > 12 && &data[..4] == b"RIFF" && &data[8..12] == b"WEBP" {
        return Some(".webp");
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn avatar_sniffing() {
        assert_eq!(infer_avatar_type(b"\x89PNG\r\n\x1a\nxxxx"), Some(".png"));
        assert_eq!(infer_avatar_type(b"\xff\xd8\xff\xe0jpeg"), Some(".jpg"));
        let webp = [
            b'R', b'I', b'F', b'F', 0, 0, 0, 0, b'W', b'E', b'B', b'P', 1, 2, 3,
        ];
        assert_eq!(infer_avatar_type(&webp), Some(".webp"));
        assert_eq!(infer_avatar_type(b"not an image"), None);
        assert_eq!(infer_avatar_type(b""), None);
    }
}
