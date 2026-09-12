import { describe, expect, it } from 'vitest';
import { guessMapping, parseCsvLine, parseCsvText } from './csv';

describe('parseCsvLine', () => {
  it('splits plain comma-separated values', () => {
    expect(parseCsvLine('2024-01-01,Coffee,-4.50')).toEqual(['2024-01-01', 'Coffee', '-4.50']);
  });

  it('keeps commas inside quoted cells', () => {
    expect(parseCsvLine('2024-01-01,"Coffee, Downtown",-4.50')).toEqual(['2024-01-01', 'Coffee, Downtown', '-4.50']);
  });

  it('unescapes doubled quotes inside a quoted cell', () => {
    expect(parseCsvLine('1,"She said ""hi""",2')).toEqual(['1', 'She said "hi"', '2']);
  });

  it('trims whitespace around cells', () => {
    expect(parseCsvLine(' a , b , c ')).toEqual(['a', 'b', 'c']);
  });
});

describe('parseCsvText', () => {
  it('returns null when there is no data row', () => {
    expect(parseCsvText('date,amount')).toBeNull();
    expect(parseCsvText('')).toBeNull();
  });

  it('splits header from all data rows', () => {
    const text = 'date,amount\n2024-01-01,-4.5\n2024-01-02,-9';
    const result = parseCsvText(text);
    expect(result?.headers).toEqual(['date', 'amount']);
    expect(result?.rows).toEqual([['2024-01-01', '-4.5'], ['2024-01-02', '-9']]);
  });
});

describe('guessMapping', () => {
  it('recognizes date-like headers', () => {
    expect(guessMapping('Transaction Date')).toBe('date');
  });

  it('recognizes amount/debit/credit headers', () => {
    expect(guessMapping('Amount')).toBe('amount');
    expect(guessMapping('Debit')).toBe('debit');
    expect(guessMapping('Credit')).toBe('credit');
  });

  it('recognizes type/direction headers', () => {
    expect(guessMapping('Transaction Type')).toBe('type');
    expect(guessMapping('Direction')).toBe('type');
  });

  it('recognizes description/name/memo headers', () => {
    expect(guessMapping('Description')).toBe('description');
    expect(guessMapping('Memo')).toBe('description');
  });

  it('recognizes bank-statement headers', () => {
    expect(guessMapping('Narration')).toBe('description');
    expect(guessMapping('Particulars')).toBe('description');
    expect(guessMapping('Withdrawal Amount')).toBe('debit');
    expect(guessMapping('Deposit Amount')).toBe('credit');
    expect(guessMapping('Closing Balance*')).toBe('ignore');
    expect(guessMapping('Value Date')).toBe('ignore');
    expect(guessMapping('Chq. / Ref No.')).toBe('ignore');
  });

  it('falls back to ignore for unrecognized headers', () => {
    expect(guessMapping('Reference #')).toBe('ignore');
  });
});
