//! Categories repository — SQL mirroring the Go backend's `repository/category.go`.

use base64::{engine::general_purpose::URL_SAFE as B64URL, Engine as _};
use serde_json::json;
use sqlx::SqlitePool;
use uuid::Uuid;

use crate::error::Result;
use crate::timex::go_ts;

#[derive(Clone)]
pub struct CategoryRepo {
    pub pool: SqlitePool,
}

pub struct CategoryRow {
    pub id: String,
    pub name: String,
    pub created_at: String,
}

pub struct ListCategoriesResult {
    pub categories: Vec<CategoryRow>,
    pub next_page_token: String,
}

impl CategoryRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    pub async fn create(&self, user_id: &str, inputs: &[CategoryCreateInput]) -> Result<Vec<CategoryRow>> {
        let mut rows = Vec::new();
        if inputs.is_empty() {
            return Ok(rows);
        }
        let mut tx = self.pool.begin().await?;
        for input in inputs {
            let id = Uuid::new_v4().to_string();
            let now = go_ts(chrono::Utc::now());
            sqlx::query("INSERT INTO categories (id, name, created_at, user_id) VALUES (?, ?, ?, ?)")
                .bind(&id)
                .bind(&input.name)
                .bind(&now)
                .bind(user_id)
                .execute(&mut *tx)
                .await?;
            rows.push(CategoryRow { id, name: input.name.clone(), created_at: now });
        }
        tx.commit().await?;
        Ok(rows)
    }

    /// Fetch one category by id (ownership-scoped).
    pub async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<CategoryRow>> {
        let row = sqlx::query_as::<_, RawCategory>(
            "SELECT id, name, created_at FROM categories WHERE id = ? AND user_id = ?",
        )
        .bind(id)
        .bind(user_id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    pub async fn update(&self, user_id: &str, inputs: &[CategoryUpdateInput]) -> Result<Vec<String>> {
        let mut errors = Vec::new();
        if inputs.is_empty() {
            return Ok(errors);
        }
        let mut tx = self.pool.begin().await?;
        for input in inputs {
            let result = sqlx::query("UPDATE categories SET name = COALESCE(NULLIF(?, ''), name) WHERE id = ? AND user_id = ?")
                .bind(&input.name)
                .bind(&input.id)
                .bind(user_id)
                .execute(&mut *tx)
                .await?;
            if result.rows_affected() == 0 {
                errors.push(format!("category {} not found", input.id));
            }
        }
        tx.commit().await?;
        Ok(errors)
    }

    pub async fn delete(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        let mut errors = Vec::new();
        if ids.is_empty() {
            return Ok(errors);
        }
        let mut tx = self.pool.begin().await?;
        for id in ids {
            let result = sqlx::query("DELETE FROM categories WHERE id = ? AND user_id = ?")
                .bind(id)
                .bind(user_id)
                .execute(&mut *tx)
                .await?;
            if result.rows_affected() == 0 {
                errors.push(format!("category {id} not found"));
            }
        }
        tx.commit().await?;
        Ok(errors)
    }

    pub async fn list(&self, user_id: &str, page_size: i64, page_token: &str) -> Result<ListCategoriesResult> {
        let mut query = String::from("SELECT id, name, created_at FROM categories WHERE user_id = ?");
        let mut bind_args: Vec<String> = vec![user_id.to_string()];

        if !page_token.is_empty() {
            if let Some(cur) = decode_cat_cursor(page_token) {
                query.push_str(" AND (name > ? OR (name = ? AND id > ?))");
                bind_args.push(cur.name.clone());
                bind_args.push(cur.name.clone());
                bind_args.push(cur.id.clone());
            }
        }

        query.push_str(" ORDER BY name ASC, id ASC");
        if page_size > 0 {
            let _ = std::fmt::Write::write_fmt(&mut query, format_args!(" LIMIT {}", page_size + 1));
        }

        let mut q = sqlx::query_as::<_, RawCategory>(&query);
        for a in &bind_args {
            q = q.bind(a);
        }
        let rows = q.fetch_all(&self.pool).await?;
        let mut categories: Vec<CategoryRow> = rows.into_iter().map(Into::into).collect();

        let mut next_token = String::new();
        let page_size_usize = usize::try_from(page_size).unwrap_or(0);
        if page_size_usize > 0 && categories.len() > page_size_usize {
            categories.truncate(page_size_usize);
            let last = categories.last().unwrap();
            next_token = encode_cat_cursor(&last.name, &last.id);
        }

        Ok(ListCategoriesResult { categories, next_page_token: next_token })
    }
}

pub struct CategoryCreateInput {
    pub name: String,
}

pub struct CategoryUpdateInput {
    pub id: String,
    pub name: String,
}

#[derive(sqlx::FromRow)]
struct RawCategory {
    id: String,
    name: String,
    created_at: String,
}

impl From<RawCategory> for CategoryRow {
    fn from(r: RawCategory) -> Self {
        Self { id: r.id, name: r.name, created_at: r.created_at }
    }
}

/// Go's category cursor is base64 JSON `{"n": name, "id": id}`.
fn encode_cat_cursor(name: &str, id: &str) -> String {
    let payload = json!({ "n": name, "id": id }).to_string();
    B64URL.encode(payload.as_bytes())
}

fn decode_cat_cursor(token: &str) -> Option<CatCursor> {
    let raw = B64URL.decode(token.as_bytes()).ok()?;
    let v: serde_json::Value = serde_json::from_slice(&raw).ok()?;
    Some(CatCursor { name: v["n"].as_str()?.to_string(), id: v["id"].as_str()?.to_string() })
}

struct CatCursor {
    name: String,
    id: String,
}
