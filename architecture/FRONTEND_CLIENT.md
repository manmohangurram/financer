# Architecture — Frontend Client

The per-service clients in `frontend/src/lib/api/client.ts` (`accounts()`, `transactions()`, `analytics()`, `apiKeys()`, etc.) are hand-written wrappers around `fetch`. The `api(method, path)` helper fills `{id}` path params and serializes GET query strings; transport helpers in `frontend/src/lib/api/transport.ts` (`setTokens`/`clearTokens`/`loadTokens`/`getAccessToken`) attach the JWT from `localStorage`. `frontend/src/lib/stores/auth.ts` calls `fetch` directly for auth. There is no generated TS client.

The API base resolves in this order: `window.__API_BASE__` (injected by the server from `FINANCER_DOMAIN_URL`), `VITE_API_URL`, then same-origin.
