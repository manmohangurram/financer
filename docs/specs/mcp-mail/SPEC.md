# MCP mail tools

## Problem

The MCP server exposed accounts, transactions, categories, rules and transfers, but not
the inbox. An agent could not see queued mail, approve/reject it, or manage mail rules —
even though the HTTP API already supported all of it.

## Scope

| Tool | Scope | Behaviour |
|---|---|---|
| `list_mailboxes` | read | Linked mailboxes (never the secret) |
| `list_mail_messages` | read | Filter by `status`, `search`, `accountId`; default limit 50, max 200; returns parsed amount/direction/date/account/merchant, not raw JSON |
| `get_mail_message` | read | One message with decoded `bodyText` and `rawText` |
| `act_on_mail` | read_write | Bulk approve/reject up to 500 items, with optional `accountId` / `categoryIds` overrides |
| `reject_all_mail` | read_write | Reject every message matching a filter (default `PENDING`), max 200; requires `"confirm": true` |
| `list_mail_rules` | read | Rules including their manual template |
| `create_mail_rule` | read_write | Rule + optional manual template (arrays per field) |
| `update_mail_rule` | read_write | Full replace by id; `template: null` removes the template |
| `delete_mail_rule` | read_write | Delete a rule; its manual template goes with it |

`read` keys get an error for every write tool, matching the existing scope behaviour.

## HTTP parity

No HTTP gaps were found: the UI/API already expose mailbox CRUD, message list/get, batch
approve/reject (`POST /api/mail/messages`), poll now/full poll, AI parse, rule CRUD and
template validation. `reject_all_mail` is an MCP convenience built on the same batch
endpoint semantics; it is intentionally filter-capped and confirm-gated.

## Notes

- Mail writes reuse `mailservice`, so template re-application, account resolution and
  book/review rules behave exactly like the UI.
- Message bodies are untrusted data; agents must treat them as data, not instructions.

## Out of scope

- Attachments.
- Mailbox credential changes over MCP.
- Template validation over MCP (`POST /api/mail/templates/validate` stays HTTP/UI).
