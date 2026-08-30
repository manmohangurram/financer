//! Categories service — business logic mirroring Go's `services/category.go`.

use std::sync::Arc;

use serde::Serialize;

use crate::error::{ApiError, Result};
use crate::repo::traits::category::{CategoryCreateInput, CategoryRow, CategoryUpdateInput};
use crate::repo::traits::CategoryRepo;
use crate::service::transaction::BulkResult;
use crate::timex::ts_rfc3339;

#[derive(Clone)]
pub struct CategoryService {
    repo: Arc<dyn CategoryRepo>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CategoryResponse {
    pub id: String,
    pub name: String,
    #[serde(rename = "createdAt")]
    pub created_at: String,
}

impl CategoryService {
    pub fn new(repo: Arc<dyn CategoryRepo>) -> Self {
        Self { repo }
    }

    pub async fn list(&self, user_id: &str, page_size: i64, page_token: &str) -> Result<(Vec<CategoryResponse>, String)> {
        let res = self.repo.list(user_id, page_size, page_token).await?;
        let items = res.categories.iter().map(category_wire).collect();
        Ok((items, res.next_page_token))
    }

    pub async fn create(&self, user_id: &str, names: &[String]) -> Result<BulkResult> {
        let inputs: Vec<_> = names.iter().filter(|n| !n.is_empty()).map(|n| CategoryCreateInput { name: n.clone() }).collect();
        match self.repo.create(user_id, &inputs).await {
            Ok(_) => Ok(BulkResult { success: true, message: "categories created successfully".to_string(), failed_ids: Vec::new(), skipped: 0 }),
            Err(e) => Ok(BulkResult { success: false, message: "some categories failed".to_string(), failed_ids: vec![e.message], skipped: 0 }),
        }
    }

    pub async fn update(&self, user_id: &str, inputs: &[(String, String)]) -> Result<BulkResult> {
        let repo_inputs: Vec<_> = inputs.iter().map(|(id, name)| CategoryUpdateInput { id: id.clone(), name: name.clone() }).collect();
        let errs = self.repo.update(user_id, &repo_inputs).await?;
        if !errs.is_empty() {
            return Ok(BulkResult { success: false, message: "some updates failed".to_string(), failed_ids: errs, skipped: 0 });
        }
        Ok(BulkResult { success: true, message: "categories updated successfully".to_string(), failed_ids: Vec::new(), skipped: 0 })
    }

    pub async fn delete(&self, user_id: &str, ids: &[String]) -> Result<BulkResult> {
        if ids.is_empty() {
            return Err(ApiError::bad_request("no ids provided"));
        }
        let errs = self.repo.delete(user_id, ids).await?;
        if !errs.is_empty() {
            return Ok(BulkResult { success: false, message: "some deletions failed".to_string(), failed_ids: errs, skipped: 0 });
        }
        Ok(BulkResult { success: true, message: "categories deleted successfully".to_string(), failed_ids: Vec::new(), skipped: 0 })
    }
}

fn category_wire(c: &CategoryRow) -> CategoryResponse {
    CategoryResponse { id: c.id.clone(), name: c.name.clone(), created_at: ts_rfc3339(&c.created_at) }
}
