# Architecture — Frontend Client

The per-service clients in `frontend/src/lib/api/client.ts` (`accounts()`, `transactions()`, `analytics()`, etc.) are hand-written wrappers around raw `fetch` (an `api(method, path)` helper fills `{id}` params and serializes GET query strings), attaching the JWT from `localStorage` via `frontend/src/lib/api/transport.ts` (`setTokens`/`clearTokens`/`loadTokens`/`getAccessToken`). `frontend/src/lib/stores/auth.ts` calls `fetch` directly for auth. Do not assume generated TS clients exist.
