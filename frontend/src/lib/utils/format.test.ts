import { describe, expect, it } from 'vitest';
import { formatCurrency, formatDate, dateToUnixSeconds } from './format';

describe('formatCurrency', () => {
  it('formats INR with en-IN grouping', () => {
    expect(formatCurrency(1234567.89)).toBe('₹12,34,567.89');
  });
  it('formats small amounts', () => {
    expect(formatCurrency(85.5)).toBe('₹85.50');
  });
  it('handles zero', () => {
    expect(formatCurrency(0)).toBe('₹0.00');
  });
});

describe('formatDate', () => {
  it('formats as DD MMM YY', () => {
    expect(formatDate('2026-08-06')).toBe('06 Aug 26');
  });
  it('treats numeric input as unix seconds', () => {
    expect(formatDate(1785024000)).toBe('26 Jul 26');
  });
});

describe('dateToUnixSeconds', () => {
  it('interprets YYYY-MM-DD as local midnight so the date round-trips', () => {
    const s = dateToUnixSeconds('2026-08-07');
    const d = new Date(s * 1000);
    expect(d.getFullYear()).toBe(2026);
    expect(d.getMonth()).toBe(7);
    expect(d.getDate()).toBe(7);
  });
});
import { defaultSpendingRange } from './format';

describe('defaultSpendingRange', () => {
  const now = new Date('2026-09-12T12:00:00Z');

  it('uses the smallest preset that contains the latest transaction', () => {
    expect(defaultSpendingRange('2026-09-10T10:00:00Z', now).range).toBe('7D');
    expect(defaultSpendingRange('2026-08-20T10:00:00Z', now).range).toBe('1M');
    expect(defaultSpendingRange('2026-06-01T10:00:00Z', now).range).toBe('6M');
    expect(defaultSpendingRange('2026-01-01T10:00:00Z', now).range).toBe('1Y');
  });

  it('falls back to a custom 1-year window when older than every preset', () => {
    const r = defaultSpendingRange('2024-08-08T10:00:00Z', now);
    expect(r.range).toBe('CUSTOM');
    expect(r.start).toBe('2024-08-08');
    expect(r.end).toBe('2025-08-08');
  });

  it('defaults to 7D for an unusable date', () => {
    expect(defaultSpendingRange('nonsense', now).range).toBe('7D');
  });
});
