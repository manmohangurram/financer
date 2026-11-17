# Manual mail templates

## Problem

AI-generated templates get some senders wrong (HDFC's UPI amount, for example). The
user needs to correct a sender by hand, without asking the model again, and keep that
correction forever.

## Model

A manual template is a set of regexes per field, keyed by the **rule's `fromPattern`**,
stored in `mail_templates` with `status = 'manual'`.

- Fields: `amount`, `debit`, `credit`, `account`, `merchant`, `date`.
- Each field is a **list** of patterns tried in order; the first that yields a value
  wins. That is how one sender's several mail shapes are covered.
- A field's value comes from the named group (`(?P<amount>…)`) or its first capture
  group; a pattern without a group falls back to the whole match.
- `amount`, `date` and at least one of `debit`/`credit` are required.
- Account and merchant are optional; a missing account pattern no longer triggers
  "account differs from regex hint".
- Templates live inside the mail rule form, next to From/Subject, because a template is
  useless without a sender.

## Priority

1. Manual templates (matched by `fromPattern` being a substring of the From address).
2. Learned/AI templates for the exact sender address.

AI generation never overwrites or competes with a manual template: `storeTemplate`
returns the manual row instead of inserting, and the manual row is never promoted,
sample-counted or disabled by learning.

## Editor

`MailRuleFormModal` → `MailTemplateEditor`:

- A switch enables the template; it is disabled until `From contains` is set.
- One row per pattern with add/remove; rows are monospace inputs.
- `Test against a mail` posts the sample to `POST /api/mail/templates/validate`, which
  returns per-pattern results (`matched`, captured `value`, `error`) plus the overall
  extraction, and the editor shows a stamp beside each row and one summary line.
- Saving re-applies rules to the queue, so a fix reaches already-queued mail.

## API

- `PUT/POST /api/mail/rules` accept `template` (arrays per field, or omitted).
- `GET /api/mail/rules` returns `template` for rules that have one.
- `POST /api/mail/templates/validate` is stateless: `{subject, body, patterns}`.

## Storage compatibility

`PatternList` unmarshals from a JSON string or an array, so existing learned/AI
templates (`"amount": "regex"`) keep working; new manual ones are written as arrays.

## Out of scope (for now)

- `{…}` shorthand sugar.
- Reordering patterns other than add/remove.
- Per-rule templates that share a sender are keyed by `fromPattern`, so two rules with
  the same sender pattern share one template.
