// Parses bank statement PDFs in the browser into transactions.
// Columns are read from the header row when it exists (Withdrawal/Deposit,
// Debit/Credit, Balance), so the same parser handles statements where the amount
// column depends on x-position as well as plain 2/3-number tables.
import * as pdfjs from 'pdfjs-dist';
import workerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url';

pdfjs.GlobalWorkerOptions.workerSrc = workerUrl;

export type StatementLayout = 'columns' | 'withdrawal-deposit' | 'amount-marker' | 'amount-delta' | 'amount-only';

export interface ParsedTxn {
  name: string;
  amount: number;
  type: 'DEBIT' | 'CREDIT';
  occurredAt: string;
  refNo?: string;
  balanceAfter?: number;
}

export interface StatementParse {
  rows: ParsedTxn[];
  opening?: number;
  closing?: number;
  credits: number;
  debits: number;
  valid: boolean;
  problem?: string;
  pages: number;
  layout: StatementLayout;
  missingRefs: number;
}

export interface StatementLine {
  text: string;
  items: Array<{ x: number; s: string }>;
}

type Direction = 'DEBIT' | 'CREDIT';
type Column = 'debit' | 'credit' | 'balance';

const HEAD = /^(\d{2}\/\d{2}\/\d{2,4})\s+(.*?)\s+(\d{2}\/\d{2}\/\d{2,4})\s+(.*)$/;
const PREFIX = /^(UPI-|NPS-|NEFT|FT-|POS\b|IMPS|ATM|MB-|VSI|ACH|SI-|BILL|TXN)/i;
const NUM = /-?\d[\d,]*\.\d{2}/g;
const MARKER = /\b(dr|cr)\b/i;
const num = (v: string) => Number(v.replace(/,/g, ''));
const round = (v: number) => Math.round(v * 100) / 100;

function toISO(d: string): string {
  const [dd, mm, yy] = d.split('/');
  const year = yy.length === 4 ? yy : `20${yy}`;
  return `${year}-${mm}-${dd}`;
}

function cleanName(narration: string): string {
  const n = narration.replace(/\s+/g, ' ').trim();
  const marker = n.search(/\b(UPI-|NPS-|NEFT CR-|FT-|POS\s|IMPS-|ATM|MB-|VSI|ACH|SI-|BILL|TXN)/i);
  const base = marker > 0 ? n.slice(marker) : n;
  let name = base;
  let m: RegExpMatchArray | null;
  if ((m = base.match(/^UPI-([^-]+)/))) name = m[1];
  else if ((m = base.match(/^NPS-([^-]+)/))) name = m[1];
  else if ((m = base.match(/^POS\s+\S+\s+(.+?)\s+\d{6,}/))) name = m[1];
  else if ((m = base.match(/^NEFT CR-[^-]+-(.+?)(?:-[A-Z0-9]{4,})?$/))) name = m[1];
  else if ((m = base.match(/^FT-\s*([^-]+)/))) name = m[1];
  else if ((m = base.match(/^IMPS-[^-]*-([^-]+)/))) name = m[1];
  else if ((m = base.match(/^ATM[^-]*-([^-]+)/))) name = m[1];
  name = name
    .replace(/[.@]/g, ' ')
    .replace(/\b\d{6,}\b/g, ' ')
    .replace(/\bOID[A-Z0-9]*\b/gi, ' ')
    .replace(/\s*-\s*/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
  return (name || base.slice(0, 60)).slice(0, 80);
}

interface ColumnHeader {
  debitX: number | null;
  creditX: number | null;
  balanceX: number | null;
}

function readHeader(lines: StatementLine[]): ColumnHeader | null {
  for (const line of lines) {
    const lower = line.text.toLowerCase();
    if (!/withdraw|deposit|debit|credit|balance/.test(lower)) continue;
    if (!/narration|description|particular|date/i.test(lower)) continue;
    const h: ColumnHeader = { debitX: null, creditX: null, balanceX: null };
    for (const item of line.items) {
      const t = item.s.toLowerCase();
      if (/withdraw|debit/.test(t) && h.debitX === null) h.debitX = item.x;
      if (/deposit|credit/.test(t) && h.creditX === null) h.creditX = item.x;
      if (/balance/.test(t) && h.balanceX === null) h.balanceX = item.x;
    }
    if (h.debitX !== null || h.creditX !== null || h.balanceX !== null) return h;
  }
  return null;
}

/** Nearest named column within a tolerance; amounts are right-aligned-ish. */
function columnOf(x: number, header: ColumnHeader): Column | null {
  // Columns start at their header label; amounts are right-aligned inside them,
  // so the header positions are boundaries rather than centres.
  const slack = 8;
  if (header.balanceX !== null && x >= header.balanceX - slack) return 'balance';
  if (header.creditX !== null && x >= header.creditX - slack) return 'credit';
  if (header.debitX !== null && x >= header.debitX - slack) return 'debit';
  return null;
}

function toLines(text: string): StatementLine[] {
  return text.split(/\r?\n/).map((line) => ({ text: line, items: [{ x: 0, s: line }] }));
}

/** Parses statement lines (text or positioned items). */
export function parseStatement(lines: StatementLine[] | string, forced?: StatementLayout): StatementParse {
  const rowsIn = typeof lines === 'string' ? toLines(lines) : lines;
  const header = readHeader(rowsIn);
  const useColumns = !forced && !!header && rowsIn.some((l) => l.items.some((i) => i.x > 0));
  const parsed: Array<{
    date: string;
    narration: string;
    debit: number;
    credit: number;
    balance?: number;
    marker: string | null;
    amountHint?: number;
    refNo: string;
  }> = [];
  let sawMarker = false;
  let sawThree = false;
  let sawDelta = false;

  let pending: string[] = [];

  for (let i = 0; i < rowsIn.length; i++) {
    const line = rowsIn[i];
    const text = line.text.trim();
    if (!text || text.startsWith('--- PAGE')) continue;
    if (/^Page\s*(No)?\.?\s*:?\s*\d+/i.test(text) || /^Date Narration/i.test(text) || /^From\s*:/i.test(text)) {
      pending = [];
      continue;
    }

    const m = text.match(HEAD);
    if (!m) {
      pending.push(...line.items.map((it) => it.s.trim()).filter(Boolean));
      continue;
    }

    const [, date, inline, , tail] = m;
    const pre = pending.join(' ').replace(/\s+/g, ' ').trim();
    pending = [];

    // The reference number sits between the date and the value date, usually as
    // a long digit run; names and dates are not candidates.
    const tokens = (inline.match(/\b[A-Za-z0-9]{6,}\b/g) || []).filter((t) => /\d/.test(t));
    tokens.sort((a, b) => b.length - a.length);
    const refNo = tokens[0] ?? '';

    const post: string[] = [];
    while (i + 1 < rowsIn.length) {
      const nextText = rowsIn[i + 1].text.trim();
      if (!nextText || nextText.startsWith('--- PAGE') || /^Page\s*(No)?\.?/i.test(nextText)) break;
      if (HEAD.test(nextText) || PREFIX.test(nextText)) break;
      if (nextText.startsWith('-') || /^[A-Z0-9]{4,}-/.test(nextText) || /^(ONE|TWO|THREE)$/i.test(nextText)) {
        post.push(nextText);
        i++;
        continue;
      }
      break;
    }

    const numbers = line.items
      .filter((it) => /^-?[\d,]+\.\d{2}$/.test(it.s.trim()))
      .map((it) => ({ x: it.x, v: num(it.s.trim()) }));

    let debit = 0;
    let credit = 0;
    let balance: number | undefined;
    let amountHint: number | undefined;

    if (useColumns && header && numbers.length > 0 && numbers.some((n) => columnOf(n.x, header) !== null)) {
      for (const n of numbers) {
        switch (columnOf(n.x, header)) {
          case 'debit':
            debit += Math.abs(n.v);
            break;
          case 'credit':
            credit += Math.abs(n.v);
            break;
          case 'balance':
            balance = n.v;
            break;
        }
      }
    } else {
      const values = (tail.match(NUM) || []).map(num);
      const marker = tail.match(MARKER)?.[1] ?? null;
      if (values.length >= 3) {
        debit = values[0];
        credit = values[1];
        balance = values[2];
        sawThree = true;
      } else if (values.length === 2) {
        const signed = values[0];
        balance = values[1];
        if (marker) {
          sawMarker = true;
          if (marker.toLowerCase() === 'cr') credit = Math.abs(signed);
          else debit = Math.abs(signed);
        } else if (signed < 0) {
          debit = Math.abs(signed);
        } else {
          // Direction will be resolved from the balance delta below.
          debit = -1;
          amountHint = Math.abs(signed);
        }
      } else if (values.length === 1 && marker) {
        sawMarker = true;
        if (marker.toLowerCase() === 'cr') credit = Math.abs(values[0]);
        else debit = Math.abs(values[0]);
      }
    }

    parsed.push({
      date: toISO(date),
      narration: [pre, inline, ...post].join(' ').replace(/\s+/g, ' ').trim(),
      debit,
      credit,
      balance,
      marker: tail.match(MARKER)?.[1] ?? null,
      amountHint,
      refNo
    });
  }

  const rows: ParsedTxn[] = [];
  let prev: number | null = null;
  let layout: StatementLayout = useColumns ? 'columns' : 'amount-only';

  for (const p of parsed) {
    let type: Direction | null = null;
    let amount = 0;

    if (p.debit >= 0 && p.credit > 0) {
      type = 'CREDIT';
      amount = p.credit;
    } else if (p.debit > 0) {
      type = 'DEBIT';
      amount = p.debit;
    } else if (p.debit === -1) {
      // Two values without a marker: use the running balance to decide.
      const hint = p.amountHint ?? 0;
      if (p.balance !== undefined && prev !== null) {
        type = p.balance >= prev ? 'CREDIT' : 'DEBIT';
        amount = Math.abs(p.balance - prev);
      } else if (p.balance !== undefined && hint > 0) {
        type = p.balance - hint >= 0 ? 'CREDIT' : 'DEBIT';
        amount = hint;
      }
      sawDelta = true;
    }

    if (!type || !amount) continue;
    rows.push({
      name: cleanName(p.narration),
      amount: round(amount),
      type,
      occurredAt: p.date,
      refNo: p.refNo || undefined,
      ...(p.balance === undefined ? {} : { balanceAfter: round(p.balance) })
    });
    if (p.balance !== undefined) prev = p.balance;
  }

  if (!useColumns) {
    if (sawMarker) layout = 'amount-marker';
    else if (sawDelta) layout = 'amount-delta';
    else if (sawThree) layout = 'withdrawal-deposit';
  }
  if (forced) layout = forced;

  const credits = round(rows.filter((r) => r.type === 'CREDIT').reduce((s, r) => s + r.amount, 0));
  const debits = round(rows.filter((r) => r.type === 'DEBIT').reduce((s, r) => s + r.amount, 0));
  const withBalance = rows.filter((r) => r.balanceAfter !== undefined);

  let problem: string | undefined;
  if (rows.length === 0) {
    problem = 'No transactions found in this PDF.';
  } else if (withBalance.length > 1) {
    let running = withBalance[0].balanceAfter as number;
    for (const t of withBalance.slice(1)) {
      const expected = round(running + (t.type === 'CREDIT' ? t.amount : -t.amount));
      if (Math.abs(expected - (t.balanceAfter as number)) > 0.01) {
        problem = `Balance breaks at ${t.occurredAt}: expected ${expected}, statement says ${t.balanceAfter}.`;
        break;
      }
      running = t.balanceAfter as number;
    }
  }

  const first = parsed[0];
  const opening = first && first.balance !== undefined && first.debit >= 0 ? round(first.balance - first.credit + first.debit) : undefined;
  const closing = withBalance.length ? withBalance[withBalance.length - 1].balanceAfter : undefined;

  if (!problem && opening !== undefined && closing !== undefined) {
    if (Math.abs(round(opening + credits - debits) - closing) > 0.01) {
      problem = `Rows do not add up: opening ${opening} + credits ${credits} − debits ${debits} ≠ closing ${closing}.`;
    }
  }

  const missingRefs = rows.filter((r) => !r.refNo).length;
  if (!problem && missingRefs > 0 && rows.length > missingRefs) {
    problem = `${missingRefs} of ${rows.length} rows have no reference number.`;
  }
  return { rows, opening, closing, credits, debits, valid: !problem, problem, pages: 1, layout, missingRefs };
}

/** Backwards-compatible text entry point. */
export function parseStatementText(text: string, forced?: StatementLayout): StatementParse {
  return parseStatement(text, forced);
}

async function pdfLines(file: File): Promise<{ lines: StatementLine[]; pages: number }> {
  const data = new Uint8Array(await file.arrayBuffer());
  const doc = await pdfjs.getDocument({ data, useSystemFonts: true, disableFontFace: true }).promise;
  const lines: StatementLine[] = [];

  for (let p = 1; p <= doc.numPages; p++) {
    const page = await doc.getPage(p);
    const content = await page.getTextContent();
    const byY = new Map<number, Array<{ x: number; s: string }>>();
    for (const item of content.items) {
      if (!('str' in item) || !item.str) continue;
      const y = Math.round(item.transform[5] * 2) / 2;
      const x = item.transform[4];
      if (!byY.has(y)) byY.set(y, []);
      byY.get(y)!.push({ x, s: item.str });
    }
    lines.push({ text: `--- PAGE ${p} ---`, items: [] });
    for (const [, items] of [...byY.entries()].sort((a, b) => b[0] - a[0])) {
      const sorted = items.sort((a, b) => a.x - b.x);
      const text = sorted.map((i) => i.s).join(' ').replace(/\s+/g, ' ').trim();
      if (text) lines.push({ text, items: sorted });
    }
  }
  return { lines, pages: doc.numPages };
}

/** Extracts a PDF and parses it. */
export async function readStatement(file: File, forced?: StatementLayout): Promise<StatementParse> {
  const { lines, pages } = await pdfLines(file);
  const out = { ...parseStatement(lines, forced), pages };
  return out;
}
