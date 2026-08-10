//! User service — profile reads, mirroring Go's `services/user.go` `GetProfile`.

use serde::Serialize;

use crate::error::{ApiError, Result};
use crate::repo::UserRepo;

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ProfileResponse {
    pub user_id: String,
    pub name: String,
    pub email: String,
    pub avatar_url: String,
}

#[derive(Clone)]
pub struct UserService {
    repo: UserRepo,
}

impl UserService {
    pub fn new(repo: UserRepo) -> Self {
        Self { repo }
    }

    pub async fn profile(&self, user_id: &str) -> Result<ProfileResponse> {
        let user = self.repo.by_id(user_id).await?
            .ok_or_else(|| ApiError::not_found("user not found"))?;
        Ok(ProfileResponse {
            user_id: user.id,
            name: user.name.unwrap_or_default(),
            email: user.email,
            avatar_url: user.avatar_url.unwrap_or_default(),
        })
    }
}
