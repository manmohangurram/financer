//! Shares category types + the backend-agnostic `CategoryRepo` trait.

use async_trait::async_trait;

use crate::error::Result;

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CategoryRow {
    pub id: String,
    pub name: String,
    pub created_at: String,
}

pub struct CategoryCreateInput {
    pub name: String,
}

pub struct CategoryUpdateInput {
    pub id: String,
    pub name: String,
}

pub struct ListCategoriesResult {
    pub categories: Vec<CategoryRow>,
    pub next_page_token: String,
}

#[async_trait]
pub trait CategoryRepo: Send + Sync {
    async fn create(&self, user_id: &str, inputs: &[CategoryCreateInput]) -> Result<Vec<CategoryRow>>;
    async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<CategoryRow>>;
    async fn update(&self, user_id: &str, inputs: &[CategoryUpdateInput]) -> Result<Vec<String>>;
    async fn delete(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>>;
    async fn list(&self, user_id: &str, page_size: i64, page_token: &str) -> Result<ListCategoriesResult>;
}
