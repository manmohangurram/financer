// Reads a bank statement PDF in the browser and turns it into transactions.
// Statements differ in columns, so the parser detects the row layout:
//   · withdrawal + deposit + balance   (HDFC style)
//   · amount + balance, direction from a Dr/Cr marker
//   · amount + balance, direction from the running balance
//   · amount + Dr/Cr only, no balance
import * as pdfjs from 'pdfjs-dist';
import workerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url';

pdfjs.GlobalWorkerOptions.workerSrc = workerUrl;

export type StatementLayout = 'withdrawal-deposit' | 'amount-marker' | 'amount-delta' | 'amount-only';

export interface ParsedTxn {
  name: string;
  amount: number;
  type: 'DEBIT' | 'CREDIT';
  occurredAt: string;
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
}

type Direction = 'DEBIT' | 'CREDIT';

const HEAD = /^(\d{2}\/\d{2}\/\d{4})\s+(.*?)\s+(\d{2}\/\d{2}\/\d{4})\s+(.*)$/;
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
  name = name.replace(/[.@]/g, ' ').replace(/\s+/g, ' ').trim();
  return (name || base.slice(0, 60)).slice(0, 80);
}

function markerToDirection(marker: string | null, amount: number): Direction | null {
  if (marker) return marker.toLowerCase() === 'dr' ? 'DEBIT' : 'CREDIT';
  if (amount < 0) return 'DEBIT';
  return null;
}

/** Parses already-extracted statement text (one visual line per row). */
export function parseStatementText(text: string, forced?: StatementLayout): StatementParse {
  const lines = text.split(/\r?\n/);
  const raw: Array<{ date: string; narration: string; tail: string }> = [];
  let pending: string[] = [];

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line || line.startsWith('--- PAGE')) continue;
    if (/^Page \d+ of \d+/.test(line) || /^Date Narration/.test(line) || /^Statement From/i.test(line)) {
      pending = [];
      continue;
    }

    const m = line.match(HEAD);
    if (!m) {
      pending.push(line);
      continue;
    }

    const [, date, inline, , tail] = m;
    const pre = pending;
    pending = [];

    // Continuation lines belong to this row until the next transaction starts.
    const post: string[] = [];
    while (i + 1 < lines.length) {
      const next = lines[i + 1].trim();
      if (!next || next.startsWith('--- PAGE') || /^Page \d+ of \d+/.test(next)) break;
      if (HEAD.test(next) || PREFIX.test(next)) break;
      if (next.startsWith('-') || /^[A-Z0-9]{4,}-/.test(next) || /^(ONE|TWO|THREE)$/i.test(next)) {
        post.push(next);
        i++;
        continue;
      }
      break;
    }

    raw.push({
      date: toISO(date),
      narration: [...pre, inline, ...post].filter(Boolean).join(' ').replace(/\s+/g, ' ').trim(),
      tail
    });
  }

  const rows: ParsedTxn[] = [];
  let detected: StatementLayout = 'amount-only';
  let prev: number | null = null;
  let firstWidth = 0;

  for (const r of raw) {
    const numbers = (r.tail.match(NUM) || []).map(num);
    const marker = r.tail.match(MARKER)?.[1] ?? null;
    let amount = 0;
    let direction: Direction | null = null;
    let balance: number | undefined;

    if (forced === 'withdrawal-deposit' || (!forced && numbers.length >= 3)) {
      amount = numbers[1] > 0 ? numbers[1] : numbers[0];
      direction = numbers[1] > 0 ? 'CREDIT' : 'DEBIT';
      balance = numbers[2];
      detected = 'withdrawal-deposit';
    } else if (forced === 'amount-marker' || (!forced && numbers.length === 2 && marker)) {
      amount = Math.abs(numbers[0]);
      direction = markerToDirection(marker, numbers[0]);
      balance = numbers[1];
      detected = 'amount-marker';
    } else if (forced === 'amount-delta' || (!forced && numbers.length === 2)) {
      amount = Math.abs(numbers[0]);
      balance = numbers[1];
      direction = markerToDirection(marker, numbers[0]);
      if (!direction && prev !== null) direction = balance >= prev ? 'CREDIT' : 'DEBIT';
      if (!direction) direction = balance - amount >= 0 ? 'CREDIT' : 'DEBIT';
      detected = 'amount-delta';
    } else {
      amount = Math.abs(numbers[0] ?? 0);
      direction = markerToDirection(marker, numbers[0] ?? 0);
      detected = 'amount-only';
    }

    if (detected === 'withdrawal-deposit') firstWidth ||= 3;
    if (detected !== 'withdrawal-deposit') firstWidth ||= 2;

    if (!amount || !direction) continue;
    rows.push({
      name: cleanName(r.narration),
      amount: round(amount),
      type: direction,
      occurredAt: r.date,
      ...(balance === undefined ? {} : { balanceAfter: round(balance) })
    });
    if (balance !== undefined) prev = balance;
  }

  const credits = round(rows.filter((r) => r.type === 'CREDIT').reduce((s, r) => s + r.amount, 0));
  const debits = round(rows.filter((r) => r.type === 'DEBIT').reduce((s, r) => s + r.amount, 0));
  const withBalance = rows.filter((r) => r.balanceAfter !== undefined);

  let problem: string | undefined;
  if (rows.length === 0) {
    problem = 'No transactions found in this PDF.';
  } else if (withBalance.length > 1) {
    // Row-to-row: each balance must equal the previous one ± the amount.
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

  const first = raw[0];
  const firstNumbers = first ? (first.tail.match(NUM) || []).map(num) : [];
  const opening =
    firstNumbers.length >= 3
      ? round(firstNumbers[2] - firstNumbers[1] + firstNumbers[0])
      : undefined;
  const closing = withBalance.length ? withBalance[withBalance.length - 1].balanceAfter : undefined;

  if (!problem && opening !== undefined && closing !== undefined) {
    if (Math.abs(round(opening + credits - debits) - closing) > 0.01) {
      problem = `Rows do not add up: opening ${opening} + credits ${credits} − debits ${debits} ≠ closing ${closing}.`;
    }
  }

  return { rows, opening, closing, credits, debits, valid: !problem, problem, pages: 1, layout: detected };
}

/** Extracts text from a PDF, preserving visual line order. */
export async function readStatement(file: File, forced?: StatementLayout): Promise<StatementParse> {
  const data = new Uint8Array(await file.arrayBuffer());
  const doc = await pdfjs.getDocument({ data, useSystemFonts: true, disableFontFace: true }).promise;
  const pages: string[] = [];

  for (let p = 1; p <= doc.numPages; p++) {
    const page = await doc.getPage(p);
    const content = await page.getTextContent();
    const rows = new Map<number, Array<{ x: number; s: string }>>();
    for (const item of content.items) {
      if (!('str' in item) || !item.str) continue;
      const y = Math.round(item.transform[5] * 2) / 2;
      const x = item.transform[4];
      if (!rows.has(y)) rows.set(y, []);
      rows.get(y)!.push({ x, s: item.str });
    }
    const lines = [...rows.entries()]
      .sort((a, b) => b[0] - a[0])
      .map(([, items]) => items.sort((a, b) => a.x - b.x).map((i) => i.s).join(' ').replace(/\s+/g, ' ').trim());
    pages.push(lines.filter(Boolean).join('\n'));
  }

  const text = pages.map((p, i) => `--- PAGE ${i + 1} ---\n${p}`).join('\n');
  return { ...parseStatementText(text, forced), pages: doc.numPages };
}
