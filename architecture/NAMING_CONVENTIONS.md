# Naming Conventions (Go)

- **Full words, no abbreviations.** Types, functions and fields spell out domain nouns: `TransactionRepo`, `TransactionService`, `AccountService`, `CategoryRow`, `EndingNumbers`. Universally-clear locals are fine (`id`, `req`, `row`, `err`, `ok`). Established short field names in repos (`txns`, `accounts`) are acceptable when the type is obvious.
- **No stutter.** A method on `TransactionService` is `Create`, not `CreateTransaction`; the package name already supplies the noun. Avoid repeating the package name in identifiers (`transactionservice.New`, not `NewTransactionService`).
- **Wire vs storage types are separate on purpose.** `internal/model` holds camelCase JSON structs; `internal/repo` holds row/input structs with plain fields. Services translate. Enum values are string constants rendered as `SCREAMING_SNAKE_CASE` on the wire.
- **Strict validation, no sentinels.** Unknown enum/wire values are a 400 at the handler/service boundary. No `Unspecified`/zero-value default.
- **SQL column and wire names are sacred.** Columns stay `snake_case` (`user_id`, `occurred_at`); JSON stays camelCase (`userId`, `occurredAt`, `type`). A rename ships as a migration in `db/migrations/`.
- **Name by role, not shape.** `repo`, `svc`, `id`, `req`, `row` are fine; `data`, `value`, `list`, `string` for a typed thing are not.
- **Booleans read as predicates** (`isLinked`, `inserted`, `success`); actions are verbs (`Create`, `Link`, `Delete`).
- **Formatting:** `gofmt`/`goimports` decide layout; files are `snake_case.go` (e.g. `account_category.go`), exported identifiers PascalCase, unexported camelCase, constants camelCase unless they are wire values.
