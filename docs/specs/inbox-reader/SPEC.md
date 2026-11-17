# Inbox Reader — SPEC

Status: approved for implementation
Owner: UI + mail ingest
Related: `docs/specs/mail-templates/SPEC.md`

## Objective

The Inbox is the page where a bank alert is read and booked. Two problems make it
harder than it should be:

1. The page is capped at 1180px and the queue/detail split leaves most of a wide
   screen unused, while the detail pane is a cramped two-column grid.
2. The "email" pane is the raw RFC 822 message: transport headers (`Delivered-To`,
   `Received`, `ARC-Seal`, `dkim-pass`) and MIME noise, with no typographic
   structure. On a phone it is unreadable.

This feature makes the Inbox a full-bleed reading surface: a parsed email on top
of the page and the booking form directly below it, so the flow is read → correct →
approve on one vertical axis.

## Behaviour

### Shell

- The content column is fluid: the shell no longer caps pages at 1180px. Pages keep
  their 24/32px gutters and define their own measure for prose.
- Queue-style pages (Inbox, transactions) use the full width; long-form text keeps
  a readable measure (≤72ch) inside its card.

### Inbox layout

- Desktop (≥1024px): two columns — a queue column (360–420px) with its own scroll,
  and a detail column that scrolls independently.
- The detail column stacks, top to bottom:
  1. **Email card** — subject, sender name and address, date, mailbox, state stamp,
     and the parsed body in a reading column. A "Original" toggle reveals the raw
     source in a monospace well.
  2. **Booking card** — name, amount, type, date, account, categories, error summary
     and the Approve / Reject actions.
- Below 1024px the layout stacks: queue first (bounded height, own scroll), then the
  email card, then the booking form.
- Scrollbars in the queue and detail columns are always visible (thin, tokenised),
  and both columns have a hairline boundary so the split is obvious.

### Email parsing

Parsing happens once, at ingest, in the mail poll:

- Decode the MIME structure of the stored message (multipart/alternative, nested
  multipart, quoted-printable, base64, charsets).
- Prefer the `text/plain` part; when only `text/html` exists, convert it to text
  (block elements become line breaks, list items become `•`, script/style/head are
  dropped, entities are decoded).
- Remove the quoted reply history (`On … wrote:`, `-----Original Message-----`,
  `________________________________`).
- Collapse runs of blank lines, trim trailing whitespace, cap the stored text at
  200k characters.
- Keep the sender display name (`from_name`) for the reader header.
- If the message cannot be parsed as MIME, fall back to stripping the leading header
  block from the raw text.
- The raw message is still stored (`raw_text`) because template extraction and LLM
  samples scan it.

### API

- `GET /api/mail/messages` is cursor-paginated (`limit` default 20, `pageToken`,\n  `accountId`, `search`) and returns `total` + `nextPageToken`; rows are **without**\n  `rawText` (payload size) and
  with `fromName` plus `bodyPreview` (first non-empty line, ≤140 chars).
- `GET /api/mail/messages/{id}` returns the full message: `bodyText` (parsed) and
  `rawText` (source). Rows stored before this feature return a lazily parsed
  `bodyText` derived from `rawText`.
- Unknown ids return `404`.

### Schema

`000003_mail_body` adds, to `mail_messages`:

| column | type | notes |
|---|---|---|
| `body_text` | TEXT NOT NULL DEFAULT '' | parsed, display-ready body |
| `from_name` | TEXT NOT NULL DEFAULT '' | decoded sender display name |

### Controls

- Form controls use daisyUI component classes (`input input-bordered`,
  `select select-bordered`, `btn btn-primary` / `btn btn-outline`, `badge`) with the
  project tokens applied through the daisyUI theme; dropdowns no longer rely on
  browser default styling.
- Type and account are daisyUI selects; categories remain toggle chips.
- States: filled, focused, invalid (debit border + message), disabled, loading.

## Non-goals

- Rendering remote HTML (no images, no scripts, no `v-html`): email HTML is
  converted to text server-side. This avoids XSS and tracking pixels.
- Attachment extraction.
- Changing template generation, rules or approval semantics.

## Acceptance

- A raw Gmail alert (headers + multipart) displays as readable prose with no
  `Received:`/`DKIM` lines and no quoted history.
- The Inbox uses the full window width at 1440px and stacks cleanly at 390px.
- The queue and detail columns both scroll independently with visible scrollbars.
- Approving from the booking card below the email still creates the transaction with
  the overridden fields.
- `GET /api/mail/messages` no longer ships raw message bodies.
