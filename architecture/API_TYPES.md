# Architecture — API Types

Wire types are hand-written Go structs in `internal/model/*.go` with `json:"camelCase"` tags; repository row types live in `internal/repo/*.go` and use plain Go fields (no JSON tags). Services map rows → wire types. Closed sets are string constants (`AccountType`, `TransactionType`, `RuleLogic`, `MatchField`, `MatchOperator`, `ActionOp`, `ActionType`) rendered as `SCREAMING_SNAKE_CASE`; an unknown value is a 400 at the boundary, never a silent default.

The OpenAPI document is hand-written in `internal/openapi/spec.go` and served at `/openapi.json`; Swagger UI is at `/docs`. Keep the spec in sync when a route or payload changes (a test asserts the documented paths).
