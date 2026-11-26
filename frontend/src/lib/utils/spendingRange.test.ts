import { describe, expect, it } from 'vitest';
import { dayCount, formatSpan, resolveSpan, spanGranularity } from './spendingRange';

const now = new Date(2026, 10, 27); // 27 Nov 2026, local

describe('resolveSpan', () => {
  it('counts 7D as today plus the six days before it', () => {
    expect(resolveSpan('7D', { from: '', to: '' }, now)).toEqual({ from: '2026-11-21', to: '2026-11-27' });
  });

  it('steps 1M, 6M and 1Y back from the same day', () => {
    expect(resolveSpan('1M', { from: '', to: '' }, now).from).toBe('2026-10-27');
    expect(resolveSpan('6M', { from: '', to: '' }, now).from).toBe('2026-05-27');
    expect(resolveSpan('1Y', { from: '', to: '' }, now).from).toBe('2025-11-27');
  });

  it('hands back the custom window untouched', () => {
    expect(resolveSpan('CUSTOM', { from: '2026-11-20', to: '2026-12-04' }, now)).toEqual({
      from: '2026-11-20',
      to: '2026-12-04'
    });
  });
});

describe('dayCount', () => {
  it('counts both ends', () => {
    expect(dayCount({ from: '2026-11-20', to: '2026-11-26' })).toBe(7);
    expect(dayCount({ from: '2026-11-27', to: '2026-11-27' })).toBe(1);
  });

  it('is zero for nonsense instead of NaN', () => {
    expect(dayCount({ from: 'nope', to: '2026-11-26' })).toBe(0);
  });
});

describe('spanGranularity', () => {
  it('charts a month or less by day', () => {
    expect(spanGranularity(resolveSpan('7D', { from: '', to: '' }, now))).toBe('day');
    expect(spanGranularity(resolveSpan('1M', { from: '', to: '' }, now))).toBe('day');
    expect(spanGranularity({ from: '2026-11-01', to: '2026-11-30' })).toBe('day');
  });

  it('charts longer windows by month', () => {
    expect(spanGranularity(resolveSpan('6M', { from: '', to: '' }, now))).toBe('month');
    expect(spanGranularity(resolveSpan('1Y', { from: '', to: '' }, now))).toBe('month');
    expect(spanGranularity({ from: '2026-11-01', to: '2027-01-05' })).toBe('month');
  });
});

describe('formatSpan', () => {
  it('reads date and month on both sides, with no year', () => {
    expect(formatSpan({ from: '2026-11-21', to: '2026-11-27' })).toBe('21 Nov – 27 Nov');
    expect(formatSpan({ from: '2026-11-20', to: '2026-12-04' })).toBe('20 Nov – 4 Dec');
  });

  it('adds a short year to both sides when the window crosses one', () => {
    expect(formatSpan({ from: '2025-11-27', to: '2026-11-27' })).toBe('27 Nov 25 – 27 Nov 26');
  });

  it('shows a single day twice rather than inventing a range', () => {
    expect(formatSpan({ from: '2026-11-27', to: '2026-11-27' })).toBe('27 Nov – 27 Nov');
  });
});
