import { describe, it, expect } from 'vitest';
import { cellToText, parseWorkbookText } from '@/lib/utils/workbook';

describe('workbook', () => {
  it('cellToText normalizes dates, numbers, and nulls', () => {
    expect(cellToText(null)).toBe('');
    expect(cellToText(undefined)).toBe('');
    expect(cellToText('  NTPC.NS  ')).toBe('NTPC.NS');
    expect(cellToText(342.5)).toBe('342.5');
    expect(cellToText(new Date(2026, 7, 9))).toBe('2026-08-09');
  });

  it('parseWorkbookText maps a CSV string into headers + rows', () => {
    const out = parseWorkbookText('Symbol,Name,Units\nNTPC.NS,NTPC,10\n');
    expect(out).toEqual({ headers: ['Symbol', 'Name', 'Units'], rows: [['NTPC.NS', 'NTPC', '10']] });
  });
});
