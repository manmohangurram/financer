# Todo — MCP mail tools

- [x] `Mail` service added to `mcpserver.Services` and wired in the router
- [x] `list_mailboxes`
- [x] `list_mail_messages` with structured parsed fields
- [x] `get_mail_message` with body
- [x] `act_on_mail` bulk approve/reject (write scope, 500 cap)
- [x] `reject_all_mail` filtered, confirm-gated, 200 cap
- [x] `list_mail_rules` / `create_mail_rule` / `update_mail_rule` / `delete_mail_rule`
- [x] Read-only refusal for mail writes
- [x] Tests for mail read and write tools
- [x] `docs/MCP.md` reference + README link
- [x] Spec, plan, todo committed with the feature
