//! `SQLite` categories repository — CRUD SQL, implementing
//! `crate::repo::traits::CategoryRepo` over an `sqlx::SqlitePool`.

use async_trait::async_trait;
use base64::{Engine as _, engine::general_purpose::URL_SAFE as B64URL};
use serde_json::json;
use sqlx::SqlitePool;
use uuid::Uuid;

use crate::error::Result;
use crate::repo::traits::category::{
    CategoryCreateInput, CategoryRow, CategoryUpdateInput, ListCategoriesResult,
};

pub struct SqliteCategoryRepo {
    pool: SqlitePool,
}

impl SqliteCategoryRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    async fn create_inner(
        &self,
        user_id: &str,
        inputs: &[CategoryCreateInput],
    ) -> Result<Vec<CategoryRow>> {
        let mut rows = Vec::new();
        for input in inputs {
            let id = Uuid::new_v4().to_string();
            let now = crate::utils::timex::now_go_ts();
            sqlx::query(
                "INSERT INTO categories (id, user_id, name, created_at) VALUES (?, ?, ?, ?)",
            )
            .bind(&id)
            .bind(user_id)
            .bind(input.name.as_str())
            .bind(&now)
            .execute(&self.pool)
            .await?;
            rows.push(CategoryRow {
                id,
                name: input.name.clone(),
                created_at: now,
            });
        }
        Ok(rows)
    }

    async fn get_by_id_inner(&self, user_id: &str, id: &str) -> Result<Option<CategoryRow>> {
        let row = sqlx::query_as::<_, RawCat>(
            "SELECT id, name, created_at FROM categories WHERE id = ? AND user_id = ? LIMIT 1",
        )
        .bind(id)
        .bind(user_id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    async fn update_inner(
        &self,
        user_id: &str,
        inputs: &[CategoryUpdateInput],
    ) -> Result<Vec<String>> {
        let mut errors = Vec::new();
        for input in inputs {
            let result = sqlx::query("UPDATE categories SET name = COALESCE(NULLIF(?, ''), name) WHERE id = ? AND user_id = ?")
                .bind(input.name.as_str())
                .bind(input.id.as_str())
                .bind(user_id)
                .execute(&self.pool)
                .await?;
            if result.rows_affected() == 0 {
                errors.push(format!("category {} not found", input.id));
            }
        }
        Ok(errors)
    }

    async fn delete_inner(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        let mut errors = Vec::new();
        for id in ids {
            let result = sqlx::query("DELETE FROM categories WHERE id = ? AND user_id = ?")
                .bind(id)
                .bind(user_id)
                .execute(&self.pool)
                .await?;
            if result.rows_affected() == 0 {
                errors.push(format!("category {id} not found"));
            }
        }
        Ok(errors)
    }

    async fn list_inner(
        &self,
        user_id: &str,
        page_size: i64,
        page_token: &str,
    ) -> Result<ListCategoriesResult> {
        let cursor = decode_cat_cursor(page_token);
        let mut sql = String::from("SELECT id, name, created_at FROM categories WHERE user_id = ?");
        if let Some(_cur) = &cursor {
            sql.push_str(" AND (name > ? OR (name = ? AND id > ?))");
        }
        sql.push_str(" ORDER BY name ASC, id ASC");
        if page_size > 0 {
            sql.push_str(" LIMIT ?");
        }
        // sqlx needs parameters in order; build binds manually.
        let page_size_usize = usize::try_from(page_size).unwrap_or(0);
        let mut q = sqlx::query_as::<_, RawCat>(&sql).bind(user_id);
        if let Some(cur) = &cursor {
            q = q
                .bind(cur.name.as_str())
                .bind(cur.name.as_str())
                .bind(cur.id.as_str());
        }
        if page_size > 0 {
            q = q.bind(page_size + 1);
        }
        let rows: Vec<RawCat> = q.fetch_all(&self.pool).await?;
        let mut categories: Vec<CategoryRow> = rows.into_iter().map(Into::into).collect();

        let mut next_token = String::new();
        if page_size_usize > 0 && categories.len() > page_size_usize {
            categories.truncate(page_size_usize);
            if let Some(last) = categories.last() {
                next_token = encode_cat_cursor(&last.name, &last.id);
            }
        }
        Ok(ListCategoriesResult {
            categories,
            next_page_token: next_token,
        })
    }
}

#[async_trait]
impl crate::repo::traits::CategoryRepo for SqliteCategoryRepo {
    async fn create(
        &self,
        user_id: &str,
        inputs: &[CategoryCreateInput],
    ) -> Result<Vec<CategoryRow>> {
        self.create_inner(user_id, inputs).await
    }
    async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<CategoryRow>> {
        self.get_by_id_inner(user_id, id).await
    }
    async fn update(&self, user_id: &str, inputs: &[CategoryUpdateInput]) -> Result<Vec<String>> {
        self.update_inner(user_id, inputs).await
    }
    async fn delete(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        self.delete_inner(user_id, ids).await
    }
    async fn list(
        &self,
        user_id: &str,
        page_size: i64,
        page_token: &str,
    ) -> Result<ListCategoriesResult> {
        self.list_inner(user_id, page_size, page_token).await
    }
}

#[derive(sqlx::FromRow)]
struct RawCat {
    id: String,
    name: String,
    created_at: String,
}

impl From<RawCat> for CategoryRow {
    fn from(r: RawCat) -> Self {
        Self {
            id: r.id,
            name: r.name,
            created_at: r.created_at,
        }
    }
}

fn encode_cat_cursor(name: &str, id: &str) -> String {
    let payload = json!({ "n": name, "id": id }).to_string();
    B64URL.encode(payload.as_bytes())
}

fn decode_cat_cursor(token: &str) -> Option<CatCursor> {
    let raw = B64URL.decode(token.as_bytes()).ok()?;
    let v: serde_json::Value = serde_json::from_slice(&raw).ok()?;
    Some(CatCursor {
        name: v["n"].as_str()?.to_string(),
        id: v["id"].as_str()?.to_string(),
    })
}

struct CatCursor {
    name: String,
    id: String,
}
