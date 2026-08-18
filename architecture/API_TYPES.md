# Architecture — API Types

API types are hand-written Rust structs in the repo/service layer, serialized by serde: `#[serde(rename_all = "camelCase")]` + `#[serde(rename)]` + `serialize_with` map Rust fields ↔ wire. Enums use `strum::Display`/`EnumString` + serde `rename_all` for the wire string (e.g. `TransactionType::Debit` ↔ `"DEBIT"`). No getters — fields are exported and accessed directly. OpenAPI spec is generated with `utoipa` (`#[utoipa::path]` on handlers, `ToSchema`/`IntoParams` on types); Swagger UI at `/docs`, spec at `/openapi.json`.
