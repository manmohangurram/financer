export function formatCurrency(amount: number): string {
  const formatter = new Intl.NumberFormat('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  return `₹${formatter.format(amount)}`;
}

export function formatDate(value: number | string): string {
  const date = typeof value === 'string' ? new Date(value) : new Date(value * 1000);
  const d = String(date.getDate()).padStart(2, '0');
  const m = date.toLocaleDateString('en-US', { month: 'short' });
  const y = String(date.getFullYear()).slice(-2);
  return `${d} ${m} ${y}`;
}

export function toLocalDateString(date: Date): string {
  const d = String(date.getDate()).padStart(2, '0');
  const m = String(date.getMonth() + 1).padStart(2, '0');
  return `${date.getFullYear()}-${m}-${d}`;
}

// Interprets a YYYY-MM-DD date as LOCAL midnight so the stored instant maps back
// to the same calendar date in any timezone (avoids UTC date-only parsing drift).
export function dateToUnixSeconds(dateStr: string): number {
  const [y, m, d] = dateStr.split('-').map(Number);
  return Math.floor(new Date(y, m - 1, d).getTime() / 1000);
}
export type SpendingRange = '7D' | '1M' | '6M' | '1Y' | 'CUSTOM';

// Smallest preset containing the latest transaction, else a 1-year custom window.
export function defaultSpendingRange(
  latestIso: string,
  now = new Date()
): { range: SpendingRange; start?: string; end?: string } {
  const d = new Date(latestIso);
  if (isNaN(d.getTime())) return { range: '7D' };
  const days = (now.getTime() - d.getTime()) / 86400000;
  if (days <= 7) return { range: '7D' };
  if (days <= 31) return { range: '1M' };
  if (days <= 183) return { range: '6M' };
  if (days <= 366) return { range: '1Y' };
  const end = new Date(d);
  end.setFullYear(end.getFullYear() + 1);
  return { range: 'CUSTOM', start: toLocalDateString(d), end: toLocalDateString(end) };
}
