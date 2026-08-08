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