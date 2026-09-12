import { describe, expect, it } from 'vitest';
import { guessInvestmentMapping } from './investmentImport';

describe('guessInvestmentMapping', () => {
  it('maps common header names', () => {
    expect(guessInvestmentMapping('Symbol')).toBe('symbol');
    expect(guessInvestmentMapping('Scheme Name')).toBe('name');
    expect(guessInvestmentMapping('Units')).toBe('quantity');
    expect(guessInvestmentMapping('NAV')).toBe('price');
    expect(guessInvestmentMapping('Trade Date')).toBe('date');
    expect(guessInvestmentMapping('Some Junk')).toBe('ignore');
  });
});
