import { roundMoney } from '@/lib/utils/money';

export function parseCsvLine(line: string): string[] {
  const cells: string[] = [];
  let cur = '';
  let inQuotes = false;
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (inQuotes) {
      if (ch === '"' && line[i + 1] === '"') { cur += '"'; i++; }
      else if (ch === '"') inQuotes = false;
      else cur += ch;
    } else if (ch === '"') inQuotes = true;
    else if (ch === ',') { cells.push(cur); cur = ''; }
    else cur += ch;
  }
  cells.push(cur);
  return cells.map((c) => c.trim());
}

export function parseCsvText(text: string): { headers: string[]; rows: string[][] } | null {
  const lines = text.split(/\r?\n/).filter((l) => l.trim().length > 0);
  if (lines.length < 2) return null;
  return { headers: parseCsvLine(lines[0]), rows: lines.slice(1).map(parseCsvLine) };
}

export type CsvField = 'ignore' | 'date' | 'description' | 'amount' | 'type' | 'debit' | 'credit';

export function guessMapping(header: string): CsvField {
  const l = header.toLowerCase();
  if (l.includes('date')) return 'date';
  if (l.includes('debit')) return 'debit';
  if (l.includes('credit')) return 'credit';
  if (l.includes('type') || l.includes('direction')) return 'type';
  if (l.includes('amount')) return 'amount';
  if (l.includes('desc') || l.includes('name') || l.includes('memo')) return 'description';
  return 'ignore';
}

export interface ImportedTransaction {
  name: string;
  amount: number;
  type: 'CREDIT' | 'DEBIT';
  accountId: string;
  occurredAt: { seconds: number; nanos: number };
  categoryIds: string[];
  externalId: string;
}

// fnv1a hash — a stable per-file fingerprint (not security, just a key) so a
// re-imported file gets the same external ids and duplicates are skipped.
function fnv1a(str: string): string {
  let h = 0x811c9dc5;
  for (let i = 0; i < str.length; i++) {
    h ^= str.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return (h >>> 0).toString(16);
}

// csvFileKey returns a stable fingerprint for a CSV's raw text.
export function csvFileKey(text: string): string {
  return fnv1a(text);
}

function parseTypeValue(v: string): 'CREDIT' | 'DEBIT' | null {
  const s = v.toLowerCase();
  if (/(credit|cr|deposit|received|income|refund|\+)/.test(s)) return 'CREDIT';
  if (/(debit|dr|withdraw|payment|expense|fee|-)/.test(s)) return 'DEBIT';
  return null;
}

function resolveAmountAndType(row: string[], idx: { type: number; debit: number; credit: number; amount: number }) {
  const debitV = idx.debit >= 0 ? parseFloat(row[idx.debit]) || 0 : 0;
  const creditV = idx.credit >= 0 ? parseFloat(row[idx.credit]) || 0 : 0;
  let typeHint = idx.type >= 0 ? parseTypeValue(row[idx.type]) : null;

  let amt = 0;
  if (debitV || creditV) {
    if (debitV && creditV) {
      const isCredit = typeHint === null ? creditV >= debitV : typeHint === 'CREDIT';
      amt = isCredit ? creditV : debitV;
      typeHint = isCredit ? 'CREDIT' : 'DEBIT';
    } else if (creditV) {
      amt = creditV;
      typeHint = 'CREDIT';
    } else {
      amt = debitV;
      typeHint = 'DEBIT';
    }
  } else if (idx.amount >= 0) {
    amt = parseFloat(row[idx.amount]) || 0;
  }
  if (typeHint === null) typeHint = amt < 0 ? 'DEBIT' : 'CREDIT';
  return { amount: Math.abs(amt), type: typeHint };
}

function parseDMY(cell: string): Date | null {
  const m = (cell || '').trim().match(/^(\d{1,2})[/-](\d{1,2})[/-](\d{2,4})$/);
  if (!m) return null;
  const day = parseInt(m[1], 10);
  const month = parseInt(m[2], 10);
  const year = parseInt(m[3], 10) < 100 ? 2000 + parseInt(m[3], 10) : parseInt(m[3], 10);
  const d = new Date(year, month - 1, day);
  if (d.getFullYear() === year && d.getMonth() === month - 1 && d.getDate() === day) return d;
  return null;
}

export function buildDate(dateCell: string): number {
  const dmy = parseDMY(dateCell);
  if (dmy) return dmy.getTime();
  const iso = (dateCell || '').trim().match(/^(\d{4})-(\d{1,2})-(\d{1,2})$/);
  if (iso) {
    const d = new Date(parseInt(iso[1], 10), parseInt(iso[2], 10) - 1, parseInt(iso[3], 10));
    if (!isNaN(d.getTime())) return d.getTime();
  }
  const d = new Date(dateCell || '');
  return isNaN(d.getTime()) ? Date.now() : d.getTime();
}

export function mapCsvRowsToTransactions(rows: string[][], mapping: string[], accountId: string, fileKey = ''): ImportedTransaction[] {
  const idx = {
    type: mapping.indexOf('type'),
    debit: mapping.indexOf('debit'),
    credit: mapping.indexOf('credit'),
    amount: mapping.indexOf('amount'),
    date: mapping.indexOf('date'),
    description: mapping.indexOf('description')
  };
  return rows
    .map((row, i) => {
      const { amount, type } = resolveAmountAndType(row, idx);
      return {
        name: idx.description >= 0 ? row[idx.description] : 'Imported',
        amount: roundMoney(amount),
        type,
        accountId,
        occurredAt: { seconds: Math.floor(buildDate(row[idx.date]) / 1000), nanos: 0 },
        categoryIds: [] as string[],
        externalId: fileKey ? `${fileKey}:${i}` : ''
      };
    })
    .filter((t) => t.name && t.amount);
}
