# Plan — MCP mail tools

1. **Wire the service.** Add `Mail *mailservice.Service` to `mcpserver.Services` and pass
   `Deps.Mail` from the router.
2. **Input types.** `listMailReq`, `mailIDReq`, `mailActReq`, `rejectAllMailReq`,
   `mailRuleReq`, `updateMailRuleReq`; rule requests map to `model.ReqMailRule`.
3. **Read tools.** `list_mailboxes`, `list_mail_messages` (returns parsed fields via
   `mailView`), `list_mail_rules`, `get_mail_message`.
4. **Write tools.** `act_on_mail` (bulk, 500 cap), `reject_all_mail` (filter + confirm,
   200 cap), `create_mail_rule`, `update_mail_rule`, `delete_mail_rule`; every write
   checks `canWrite()`.
5. **Docs.** `docs/MCP.md` reference (tools, args, scopes, examples), README link, spec.
6. **Tests.** Read tools under a `read` key (and read-only refusal for `act_on_mail`);
   write tools under `read_write` (rule + template, bulk reject, confirm gate, delete).
