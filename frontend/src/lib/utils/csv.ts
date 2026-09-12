// CSV helpers still used by the investments import (the transaction import now
// parses on the backend). `guessMapping` drives the header-driven mapping UI.

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
  // A running balance is not an amount; a value date is a secondary date.
  if (l.includes('balance')) return 'ignore';
  if (l.includes('value date')) return 'ignore';
  if (l.includes('withdrawal') || l.includes('debit') || l.includes('paid out')) return 'debit';
  if (l.includes('deposit') || l.includes('credit') || l.includes('paid in')) return 'credit';
  if (l.includes('narration') || l.includes('particulars') || l.includes('details') || l.includes('remarks')) return 'description';
  if (l.includes('desc') || l.includes('name') || l.includes('memo')) return 'description';
  if (l.includes('type') || l.includes('direction')) return 'type';
  if (l.includes('date')) return 'date';
  if (l.includes('amount')) return 'amount';
  return 'ignore';
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
