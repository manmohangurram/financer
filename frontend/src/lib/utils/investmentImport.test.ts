import { describe, it, expect } from 'vitest';
import { locateInvestmentTable, guessInvestmentMapping, mapImportRows } from '@/lib/utils/investmentImport';

describe('investmentImport', () => {
  it('locateInvestmentTable finds the header row amid leading junk', () => {
    const sheet = [
      ['Groww Statement'],
      ['Generated on 1 Aug 2026'],
      [],
      ['Name', 'Units', 'NAV', 'Amount'],
      ['NTPC Ltd', '10', '342.5', '3425'],
      ['Reliance', '5', '2500', '12500'],
      [],
      ['Some footer text']
    ];
    const table = locateInvestmentTable(sheet)!;
    expect(table.headers).toEqual(['Name', 'Units', 'NAV', 'Amount']);
    expect(table.data).toEqual([
      ['NTPC Ltd', '10', '342.5', '3425'],
      ['Reliance', '5', '2500', '12500']
    ]);
  });

  it('locateInvestmentTable stops at the first blank row', () => {
    const sheet = [
      ['Name', 'Quantity'],
      ['A', '1'],
      ['', ''],
      ['B', '2']
    ];
    const table = locateInvestmentTable(sheet)!;
    expect(table.data).toEqual([['A', '1']]);
  });

  it('guessInvestmentMapping maps common header names', () => {
    expect(guessInvestmentMapping('Symbol')).toBe('symbol');
    expect(guessInvestmentMapping('Quantity')).toBe('quantity');
    expect(guessInvestmentMapping('NAV')).toBe('price');
    expect(guessInvestmentMapping('Scheme Name')).toBe('name');
    expect(guessInvestmentMapping('Date')).toBe('date');
    expect(guessInvestmentMapping('junk')).toBe('ignore');
  });

  it('mapImportRows builds rows with sensible defaults', () => {
    const mapping = ['symbol', 'name', 'quantity', 'price', 'date'];
    const rows = mapImportRows([['NTPC.NS', 'NTPC Ltd', '10', '342.5', '2026-08-01']], mapping, 'file1');
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({
      symbol: 'NTPC.NS',
      name: 'NTPC Ltd',
      investmentType: 'INVESTMENT_TYPE_STOCK',
      side: 1,
      quantity: 10,
      price: 342.5,
      externalId: 'file1:0'
    });
  });
});
