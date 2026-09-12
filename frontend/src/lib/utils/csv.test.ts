import { describe, expect, it } from 'vitest';
import { guessMapping } from './csv';

describe('guessMapping', () => {
  it('recognizes date-like headers', () => {
    expect(guessMapping('Transaction Date')).toBe('date');
  });

  it('recognizes amount/debit/credit headers', () => {
    expect(guessMapping('Amount')).toBe('amount');
    expect(guessMapping('Debit')).toBe('debit');
    expect(guessMapping('Credit')).toBe('credit');
  });

  it('recognizes description headers', () => {
    expect(guessMapping('Description')).toBe('description');
    expect(guessMapping('Memo')).toBe('description');
  });

  it('recognizes bank-statement headers', () => {
    expect(guessMapping('Narration')).toBe('description');
    expect(guessMapping('Withdrawal Amount')).toBe('debit');
    expect(guessMapping('Deposit Amount')).toBe('credit');
    expect(guessMapping('Closing Balance*')).toBe('ignore');
    expect(guessMapping('Value Date')).toBe('ignore');
  });

  it('falls back to ignore for unrecognized headers', () => {
    expect(guessMapping('Reference #')).toBe('ignore');
  });
});
