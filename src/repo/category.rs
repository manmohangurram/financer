//! Categories repository — CRUD on `SurrealDB`, mirroring the Go backend semantics.

use base64::{engine::general_purpose::URL_SAFE as B64URL, Engine as _};
use serde_json::json;
use surrealdb::Connection;

use crate::error::Result;
use crate::repo::surreal::{rid, take_json, DbClient, RepoConn};
use crate::timex::go_ts;

#[derive(Clone)]
pub struct CategoryRepo<C: Connection = DbClient> {
    db: RepoConn<C>,
}

impl<C: Connection> CategoryRepo<C> {
    pub fn new(db: RepoConn<C>) -> Self {
        Self { db }
    }

    pub async fn create(&self, user_id: &str, inputs: &[CategoryCreateInput]) -> Result<Vec<CategoryRow>> {
        let mut rows = Vec::new();
        for input in inputs {
            let now = go_ts(chrono::Utc::now());
            let mut res = self
                .db
                .query(
                    "CREATE category CONTENT { user: $uid, name: $name, createdAt: $created }
                     RETURN meta::id(id) AS id, name, createdAt",
                )
                .bind(("uid", rid("user", user_id)))
                .bind(("name", input.name.clone()))
                .bind(("created", now.clone()))
                .await?
                .check()?;
            let row = take_json::<CategoryRow>(&mut res, 0)?
                .into_iter()
                .next()
                .ok_or_else(|| crate::error::ApiError::internal("category create returned no row"))?;
            rows.push(CategoryRow { created_at: now, ..row });
        }
        Ok(rows)
    }

    /// Fetch one category by id (ownership-scoped).
    pub async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<CategoryRow>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, name, createdAt FROM category
                 WHERE id = $rid AND user = $uid LIMIT 1",
            )
            .bind(("rid", rid("category", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(take_json(&mut res, 0)?.into_iter().next())
    }

    pub async fn update(&self, user_id: &str, inputs: &[CategoryUpdateInput]) -> Result<Vec<String>> {
        let mut errors = Vec::new();
        for input in inputs {
            let mut res = self
                .db
                .query(
                    "UPDATE $rid SET name = IF $name != '' THEN $name ELSE name END
                     WHERE user = $uid RETURN meta::id(id) AS id",
                )
                .bind(("rid", rid("category", &input.id)))
                .bind(("uid", rid("user", user_id)))
                .bind(("name", input.name.clone()))
                .await?;
            if take_json::<serde_json::Value>(&mut res, 0)?.is_empty() {
                errors.push(format!("category {} not found", input.id));
            }
        }
        Ok(errors)
    }

    pub async fn delete(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        let mut errors = Vec::new();
        for id in ids {
            let mut res = self
                .db
                .query("DELETE $rid WHERE user = $uid RETURN BEFORE")
                .bind(("rid", rid("category", id)))
                .bind(("uid", rid("user", user_id)))
                .await?;
            if take_json::<serde_json::Value>(&mut res, 0)?.is_empty() {
                errors.push(format!("category {id} not found"));
            }
        }
        Ok(errors)
    }

    pub async fn list(&self, user_id: &str, page_size: i64, page_token: &str) -> Result<ListCategoriesResult> {
        let mut query = String::from(
            "SELECT meta::id(id) AS id, name, createdAt FROM category WHERE user = $uid",
        );
        let cursor = decode_cat_cursor(page_token);

        if let Some(_cur) = &cursor {
            query.push_str(" AND (name > $name OR (name = $name AND id > $rid))");
        }
        query.push_str(" ORDER BY name ASC, id ASC");
        if page_size > 0 {
            let _ = std::fmt::Write::write_fmt(&mut query, format_args!(" LIMIT {}", page_size + 1));
        }

        let mut q = self.db.query(&query).bind(("uid", rid("user", user_id)));
        if let Some(cur) = &cursor {
            q = q.bind(("name", cur.name.clone())).bind(("rid", rid("category", &cur.id)));
        }
        let mut res = q.await?;
        let mut categories: Vec<CategoryRow> = take_json(&mut res, 0)?;

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

#[cfg(test)]
mod tests {
    use std::sync::Arc;
    use surrealdb::Surreal;

    use super::*;
    use crate::surreal_db;

    async fn repo() -> CategoryRepo<surrealdb::engine::local::Db> {
        let db = Arc::new(surreal_db::connect_mem().await.unwrap());
        db.query("CREATE user CONTENT { email: 'u1@x.com', passwordHash: 'h', name: 'u1' }")
            .await
            .unwrap()
            .check()
            .unwrap();
        CategoryRepo::new(db)
    }

    #[tokio::test]
    async fn create_list_update_delete() {
        let repo = repo().await;
        let created = repo.create("u1", &[CategoryCreateInput { name: "Food".into() }, CategoryCreateInput { name: "Travel".into() }]).await.unwrap();
        assert_eq!(created.len(), 2);
        assert!(!created[0].id.is_empty());

        let listed = repo.list("u1", 0, "").await.unwrap();
        assert_eq!(listed.categories.len(), 2);

        let errs = repo.update("u1", &[CategoryUpdateInput { id: created[0].id.clone(), name: "Groceries".into() }]).await.unwrap();
        assert!(errs.is_empty());
        let by_id = repo.get_by_id("u1", &created[0].id).await.unwrap().unwrap();
        assert_eq!(by_id.name, "Groceries");

        // cross-user scoping: u2 (no account) sees nothing
        assert!(repo.get_by_id("u2", &created[0].id).await.unwrap().is_none());
        let errs = repo.delete("u1", &[created[0].id.clone()]).await.unwrap();
        assert!(errs.is_empty());
        assert!(repo.get_by_id("u1", &created[0].id).await.unwrap().is_none());
    }
}
