# Architecture — Request Flow

`cmd/financer/main.go` loads config, opens the SQLite pools (applying migrations), constructs repositories → services, then builds the Gin router in `internal/server/router.go`.

- Public routes: `/api/auth/{signup,login,refresh}` (HTTPS-guarded). Everything else under `/api` goes through the bearer-JWT middleware, which validates the token and stores the user id in the Gin context.
- Per-feature handlers live in `internal/server/*.go`; routes are registered per method in `router.go` (`/api/transactions` handles GET/POST/PUT/DELETE on the same path).
- `/mcp` is a separate streamable-HTTP handler authenticated by a per-user API key (not the JWT); `/docs` and `/openapi.json` serve the API description; everything else falls back to the SPA.
