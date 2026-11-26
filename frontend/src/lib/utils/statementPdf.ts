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
  autoCorrected?: boolean;
  /** Printed columns found in the statement header (empty when there is none). */
  columns: StatementColumn[];
  /** The mapping actually used to read the rows. */
  mapping: ColumnMapping | null;
}

export interface StatementLine {
  text: string;
  items: Array<{ x: number; s: string }>;
}

/** One column printed in the statement's own header row. */
export interface StatementColumn {
  key: string;
  label: string;
  x: number;
}

export type ColumnRole = 'date' | 'name' | 'ref' | 'debit' | 'credit' | 'amount' | 'balance' | 'ignore';

/**
 * How the reader should treat each printed column. `two` reads a Debit and a
 * Credit column; `one` reads one amount column and takes the direction from a
 * Dr/Cr marker or from the change in the running balance.
 */
export interface ColumnMapping {
  mode: 'one' | 'two';
  date: string;
  name: string;
  ref: string;
  debit?: string;
  credit?: string;
  amount?: string;
  balance: string;
  direction?: 'marker' | 'balance';
}

export interface StatementDocument {
  lines: StatementLine[];
  pages: number;
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
  refX: number | null;
  debitX: number | null;
  creditX: number | null;
  balanceX: number | null;
}

function findHeaderLine(lines: StatementLine[]): StatementLine | null {
  for (const line of lines) {
    const lower = line.text.toLowerCase();
    if (!/withdraw|deposit|debit|credit|balance/.test(lower)) continue;
    if (!/narration|description|particular|date/i.test(lower)) continue;
    if (line.items.some((i) => /withdraw|deposit|debit|credit|balance/.test(i.s.toLowerCase()))) return line;
  }
  return null;
}

function readHeader(lines: StatementLine[]): ColumnHeader | null {
  const line = findHeaderLine(lines);
  if (!line) return null;
  const h: ColumnHeader = { refX: null, debitX: null, creditX: null, balanceX: null };
  for (const item of line.items) {
    const t = item.s.toLowerCase();
    if (/chq|ref/.test(t) && h.refX === null) h.refX = item.x;
    if (/withdraw|debit/.test(t) && h.debitX === null) h.debitX = item.x;
    if (/deposit|credit/.test(t) && h.creditX === null) h.creditX = item.x;
    if (/balance/.test(t) && h.balanceX === null) h.balanceX = item.x;
  }
  if (h.debitX !== null || h.creditX !== null || h.balanceX !== null) return h;
  return null;
}

/**
 * The columns printed in the statement's own header, left to right. These are
 * what the Map step lets the reader reassign; the keys A… are stable for the
 * lifetime of one statement.
 */
export function detectColumns(lines: StatementLine[]): StatementColumn[] {
  const line = findHeaderLine(lines);
  if (!line) return [];
  return line.items
    .filter((i) => i.s.trim())
    .sort((a, b) => a.x - b.x)
    .map((i, index) => ({
      key: String.fromCharCode(65 + index),
      label: i.s.trim(),
      x: i.x
    }));
}

/** The mapping the printed headers imply. */
export function defaultMapping(columns: StatementColumn[]): ColumnMapping | null {
  if (columns.length === 0) return null;
  const pick = (re: RegExp) => columns.find((c) => re.test(c.label));
  const date = pick(/^date\b/i) ?? columns[0];
  const name = pick(/narration|description|particular/i);
  const ref = pick(/chq|ref/i);
  const debit = pick(/withdraw|debit/i);
  const credit = pick(/deposit|credit/i);
  const amount = pick(/^amount/i);
  const balance = pick(/balance/i);
  if (!date || !balance) return null;
  const base = {
    date: date.key,
    name: name?.key ?? '',
    ref: ref?.key ?? '',
    balance: balance.key
  };
  if (debit && credit) {
    return { ...base, mode: 'two', debit: debit.key, credit: credit.key };
  }
  const amountCol = amount ?? debit ?? credit;
  if (!amountCol) return null;
  return { ...base, mode: 'one', amount: amountCol.key, direction: 'balance' };
}

interface Positions {
  refX: number | null;
  debitX: number | null;
  creditX: number | null;
  balanceX: number | null;
  amountX: number | null;
  mode: 'one' | 'two';
  direction: 'marker' | 'balance';
}

function positionsFor(mapping: ColumnMapping, columns: StatementColumn[]): Positions {
  const x = (key?: string) => columns.find((c) => c.key === key)?.x ?? null;
  return {
    refX: x(mapping.ref),
    debitX: mapping.mode === 'two' ? x(mapping.debit) : null,
    creditX: mapping.mode === 'two' ? x(mapping.credit) : null,
    balanceX: x(mapping.balance),
    amountX: mapping.mode === 'one' ? x(mapping.amount) : null,
    mode: mapping.mode,
    direction: mapping.direction ?? 'balance'
  };
}

/** The role of a right-aligned amount: the rightmost column that starts at or before it. */
function amountRole(x: number, p: Positions): 'debit' | 'credit' | 'balance' | 'amount' | null {
  const starts: Array<[number, 'debit' | 'credit' | 'balance' | 'amount']> = [];
  if (p.debitX !== null) starts.push([p.debitX, 'debit']);
  if (p.creditX !== null) starts.push([p.creditX, 'credit']);
  if (p.amountX !== null) starts.push([p.amountX, 'amount']);
  if (p.balanceX !== null) starts.push([p.balanceX, 'balance']);
  starts.sort((a, b) => a[0] - b[0]);
  let hit: 'debit' | 'credit' | 'balance' | 'amount' | null = null;
  for (const [start, role] of starts) {
    if (x >= start - 8) hit = role;
  }
  return hit;
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
function parseOnce(lines: StatementLine[] | string, forced?: StatementLayout, mapping?: ColumnMapping): StatementParse {
  const rowsIn = typeof lines === 'string' ? toLines(lines) : lines;
  const columns = detectColumns(rowsIn);
  const activeMapping = mapping ?? (!forced ? defaultMapping(columns) : null);
  const positions = activeMapping ? positionsFor(activeMapping, columns) : null;
  const header = readHeader(rowsIn);
  const hasPositions = rowsIn.some((l) => l.items.some((i) => i.x > 0));
  const useColumns = !!positions && hasPositions;
  const useLegacyColumns = !useColumns && !forced && !!header && hasPositions;
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

    // Prefer the reference column's own x-position (right under Chq./Ref.No.);
    // fall back to the longest digit-bearing token in the narration.
    let refNo = '';
    const refX = useColumns ? (positions?.refX ?? null) : useLegacyColumns ? (header?.refX ?? null) : null;
    if (refX !== null) {
      const hit = line.items.find((it) => Math.abs(it.x - refX) <= 40 && /^[A-Za-z0-9]{6,}$/.test(it.s.trim()));
      if (hit) refNo = hit.s.trim();
    }
    if (!refNo) {
      const tokens = (inline.match(/\b[A-Za-z0-9]{6,}\b/g) || []).filter((t) => /\d/.test(t));
      tokens.sort((a, b) => b.length - a.length);
      refNo = tokens[0] ?? '';
    }

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
    let mapped = false;

    if (useColumns && positions && numbers.length > 0) {
      let amountValue: number | undefined;
      for (const n of numbers) {
        const role = amountRole(n.x, positions);
        if (!role) continue;
        mapped = true;
        if (positions.mode === 'two') {
          if (role === 'debit') debit += Math.abs(n.v);
          else if (role === 'credit') credit += Math.abs(n.v);
          else if (role === 'balance') balance = n.v;
        } else if (role === 'amount') {
          amountValue = n.v;
        } else if (role === 'balance') {
          balance = n.v;
        }
      }
      if (positions.mode === 'one' && amountValue !== undefined) {
        const marker = text.match(MARKER)?.[1] ?? null;
        if (positions.direction === 'marker' && marker) {
          sawMarker = true;
          if (marker.toLowerCase() === 'cr') credit = Math.abs(amountValue);
          else debit = Math.abs(amountValue);
        } else {
          // Direction is resolved from the running balance below.
          debit = -1;
          amountHint = Math.abs(amountValue);
        }
      }
    } else if (
      useLegacyColumns &&
      header &&
      numbers.length > 0 &&
      numbers.some((n) => columnOf(n.x, header) !== null)
    ) {
      mapped = true;
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
    }

    if (!mapped) {
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
  let layout: StatementLayout = 'amount-only';
  if (useColumns) {
    layout =
      positions?.mode === 'one' ? (positions.direction === 'marker' ? 'amount-marker' : 'amount-delta') : 'columns';
  }

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

  const firstRow = rows[0];
  const opening =
    firstRow && firstRow.balanceAfter !== undefined
      ? round(firstRow.balanceAfter - (firstRow.type === 'CREDIT' ? firstRow.amount : -firstRow.amount))
      : undefined;
  const closing = withBalance.length ? withBalance[withBalance.length - 1].balanceAfter : undefined;

  if (!problem && opening !== undefined && closing !== undefined) {
    if (Math.abs(round(opening + credits - debits) - closing) > 0.01) {
      problem = `Rows do not add up: opening ${opening} + credits ${credits} − debits ${debits} ≠ closing ${closing}.`;
    }
  }

  const missingRefs = rows.filter((r) => !r.refNo).length;
  return {
    rows,
    opening,
    closing,
    credits,
    debits,
    valid: !problem,
    problem,
    pages: 1,
    layout,
    missingRefs,
    columns,
    mapping: useColumns ? activeMapping : null
  };
}

export function parseStatement(
  lines: StatementLine[] | string,
  forced?: StatementLayout,
  mapping?: ColumnMapping
): StatementParse {
  const rowsIn = typeof lines === 'string' ? toLines(lines) : lines;
  const header = readHeader(rowsIn);
  const columnsUsable =
    !!header && (header.debitX !== null || header.creditX !== null) && rowsIn.some((l) => l.items.some((i) => i.x > 0));

  // An explicit mapping is authoritative: re-read with exactly those columns so
  // a wrong choice shows up as a balance break rather than silently "fixing"
  // itself. Automatic detection still wins over the legacy preset layouts.
  if (mapping) {
    const explicit = parseOnce(rowsIn, undefined, mapping);
    return columnsUsable && forced && forced !== explicit.layout ? { ...explicit, autoCorrected: true } : explicit;
  }
  if (columnsUsable) {
    const auto = parseOnce(rowsIn, undefined);
    if (auto.valid && auto.rows.length > 0) {
      return forced && forced !== auto.layout ? { ...auto, autoCorrected: true } : auto;
    }
  }
  return parseOnce(rowsIn, forced);
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
      const text = sorted
        .map((i) => i.s)
        .join(' ')
        .replace(/\s+/g, ' ')
        .trim();
      if (text) lines.push({ text, items: sorted });
    }
  }
  return { lines, pages: doc.numPages };
}

/** Extracts a PDF once so later column changes can re-read without reparsing the file. */
export async function readStatementDocument(file: File): Promise<StatementDocument> {
  return pdfLines(file);
}

/** Parses an already-extracted document with an explicit column mapping. */
export function parseDocument(doc: StatementDocument, mapping?: ColumnMapping): StatementParse {
  return { ...parseStatement(doc.lines, undefined, mapping), pages: doc.pages };
}

/** Extracts a PDF and parses it. */
export async function readStatement(
  file: File,
  forced?: StatementLayout,
  mapping?: ColumnMapping
): Promise<StatementParse> {
  const { lines, pages } = await pdfLines(file);
  const out = { ...parseStatement(lines, forced, mapping), pages };
  return out;
}
