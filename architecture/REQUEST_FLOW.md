# Architecture — Request Flow

`src/main.rs` opens the DB, runs migrations, constructs repositories → services, then registers HTTP handlers via `src/http/mod.rs` `router()`. REST JSON under `/api/...`; routes per feature in `src/http/<feature>.rs` via axum `Router::route`; path params via `Path`/`Path(String)` extractors. JWT auth via `require_user(headers, &jwt)`; only `/api/auth/{signup,login,refresh}` are public.
