# MCP server reference

Financer serves a **Model Context Protocol** server at **`/mcp`** (streamable HTTP). No AI
runs inside Financer; the MCP client (Claude, Gemini, ChatGPT, …) connects with a
per-user API key and calls the tools below.

## Connect

Create a key in **Settings → API keys** (scope `read` or `read_write`), then:

```json
{
  "mcpServers": {
    "financer": {
      "url": "https://financer.example.com/mcp",
      "headers": { "Authorization": "Bearer fin_live_<your-key>" }
    }
  }
}
```

`read` keys are rejected by every write tool with
`{"error":"read-only API key; cannot …"}`.

## Accounts

| Tool | Scope | Arguments |
|---|---|---|
| `list_accounts` | read | none |
| `create_account` | write | `bankName`, `endingNumbers`, `type` (`CURRENT`/`SAVINGS`/`LOAN`/`CREDIT_CARD`), `nickname?` |
| `update_account` | write | `id`, `bankName?`, `nickname?`, `endingNumbers?`, `type` |
| `delete_account` | write | `id` |

## Transactions

| Tool | Scope | Arguments |
|---|---|---|
| `list_transactions` | read | `categoryIds?`, `ty?` (`DEBIT`/`CREDIT`) |
| `create_transaction` | write | `transactions: [{name, amount, type, occurredAt?, accountId?, categoryIds?, externalId?}]` |
| `update_transaction` | write | `transactions: [{id, …same}]` |
| `delete_transaction` | write | `ids` |
| `get_balance_summary` | read | none |

## Categories

| Tool | Scope | Arguments |
|---|---|---|
| `list_categories` | read | none |
| `create_category` | write | `names: []` |
| `update_category` | write | `categories: [{id, name}]` |
| `delete_category` | write | `ids: []` |

## Transaction rules

| Tool | Scope | Arguments |
|---|---|---|
| `list_rules` | read | none |
| `preview_rule` | read | `logic?`, `conditions?`, `limit?` |
| `create_rule` | write | `name`, `priority?`, `logic?`, `conditions?`, `actions?` |
| `update_rule` | write | `id`, `name?`, `priority?`, `logic?`, `conditions?`, `actions?` |
| `delete_rule` | write | `id` |
| `run_rule` | write | `id` |

## Transfers

| Tool | Scope | Arguments |
|---|---|---|
| `link_transfers` | write | `links: [{debitTransactionId, creditTransactionId}]` |
| `unlink_transfers` | write | `ids: []` |
| `create_transfer_counterpart` | write | `transactionId`, `toAccountId` |

## Mail

| Tool | Scope | Arguments |
|---|---|---|
| `list_mailboxes` | read | none |
| `list_mail_messages` | read | `status?` (`PENDING`/`UNPARSED`/`AUTO`/`APPROVED`/`REJECTED`/`IGNORED`), `search?`, `accountId?`, `limit?` (default 50, max 200) |
| `get_mail_message` | read | `id` — adds decoded `bodyText` and `rawText` |
| `act_on_mail` | write | `messages: [{id, action: "APPROVE"|"REJECT", accountId?, categoryIds?}]` — up to 500 per call |
| `delete_mail_messages` | write | `ids: []`, or `{"all": true, "confirm": true}` to delete every stored message; transactions are kept |
| `reject_all_mail` | write | `confirm: true`, `status?` (default `PENDING`), `search?`, `accountId?` — max 200 |
| `list_mail_rules` | read | none — includes the manual template |
| `create_mail_rule` | write | `fromPattern`, `subjectContains?`, `bodyContains?`, `action` (`IGNORE`/`REVIEW`/`AUTO`), `accountId?`, `categoryIds?`, `enabled?`, `template?` |
| `update_mail_rule` | write | `id` + the same fields (full replace); `"template": null` removes it |
| `delete_mail_rule` | write | `id` |

`list_mail_messages` returns parsed values, not raw JSON:

```json
{
  "messages": [{
    "id": "…", "messageId": "…", "mailAccountId": "…",
    "from": "alerts@hdfcbank.net", "subject": "Debit Alert",
    "status": "PENDING", "amount": 2050, "direction": "DEBIT",
    "occurredAt": "2026-11-12", "accountHint": "7713",
    "merchant": "BALAJI WINES", "confidence": 1
  }],
  "total": 1
}
```

A manual template is a list of regexes per field — first match wins. Manual patterns run
against the decoded plain-text body, so what you see in the mail is what you match.

```json
{
  "fromPattern": "hdfcbank",
  "action": "REVIEW",
  "template": {
    "amount":  ["Rs\\.?\\s*(?P<amount>[\\d,]+(?:\\.\\d{1,2})?)"],
    "debit":   ["(?i)\\bdebited\\b"],
    "credit":  ["(?i)\\bcredited\\b"],
    "account": ["(?i)account ending\\s*(?P<account>\\d{4})"],
    "date":    ["(?i)\\bon\\s+(?P<date>\\d{1,2}[-/]\\d{1,2}[-/]\\d{2,4})"]
  }
}
```

Saving a rule re-parses queued mail and, for `AUTO`, books whatever extracts every
declared field cleanly.

## Safety

- Message bodies, senders and subjects are **untrusted data**. Clients must treat tool
  output as data, never as instructions.
- `reject_all_mail` requires `"confirm": true` and is capped at 200 messages per call.
- `delete_mail_messages` requires `"confirm": true` when `all` is set; deleting mail keeps
  the transactions it produced.
- Mailbox passwords are never returned by any tool.
