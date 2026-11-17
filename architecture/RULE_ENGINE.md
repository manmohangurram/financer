# Architecture — Rule Engine

`internal/ruleservice/` implements behavior rules. A rule has a logic mode (`OR`/`AND`), conditions (field: name/amount/type/category/account; operator: contains/starts_with/ends_with/equals/gt/lt/regex; case-insensitive where relevant) and **output actions**: rename (`RENAME`/`ADD_PREFIX`/`ADD_SUFFIX`), set category, or transfer-to-account.

- Rules run **at write time**: on transaction create, the overlay (`RuleService.Overlay`) matches the new rows and snapshots the resolved display name into `clean_name` and the chosen category into the join table. The raw bank `name` is never overwritten; reads return `clean_name` when present.
- `SET_TRANSFER_ACCOUNT` is handled by `internal/ruleservice/transferrule.go`: on create it finds or creates the counterpart transaction in the target account and links them (`ResolveTransfer`).
- `POST /api/rules/{id}/run` applies one rule to existing rows; `POST /api/rules/preview` matches conditions without mutating.
- Rule CRUD lives in `internal/ruleservice/service.go` + `internal/repo/sqlite/rule.go`; creating or updating a rule backfills existing transactions.
