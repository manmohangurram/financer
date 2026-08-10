import { describe, expect, it } from 'vitest';
import { guessMapping, mapCsvRowsToTransactions, parseCsvLine, parseCsvText } from './csv';

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

  it('falls back to ignore for unrecognized headers', () => {
    expect(guessMapping('Reference #')).toBe('ignore');
  });
});

describe('mapCsvRowsToTransactions', () => {
  const mapping = ['date', 'description', 'amount'];

  it('maps a positive amount to a credit transaction', () => {
    const [txn] = mapCsvRowsToTransactions([['2024-01-01', 'Paycheck', '1500']], mapping, 'acc-1');
    expect(txn.type).toBe('CREDIT');
    expect(txn.amount).toBe(1500);
    expect(txn.name).toBe('Paycheck');
    expect(txn.accountId).toBe('acc-1');
  });

  it('maps a negative amount to a debit transaction with an absolute amount', () => {
    const [txn] = mapCsvRowsToTransactions([['2024-01-01', 'Coffee', '-4.5']], mapping, 'acc-1');
    expect(txn.type).toBe('DEBIT');
    expect(txn.amount).toBe(4.5);
  });

  it('drops rows with no amount or no name', () => {
    const rows = mapCsvRowsToTransactions(
      [
        ['2024-01-01', '', '0'],
        ['2024-01-01', 'Coffee', '']
      ],
      mapping,
      'acc-1'
    );
    expect(rows).toHaveLength(0);
  });

  it('falls back to "Imported" when description is not mapped', () => {
    const [txn] = mapCsvRowsToTransactions([['2024-01-01', '5']], ['date', 'amount'], 'acc-1');
    expect(txn.name).toBe('Imported');
  });

  it('uses a dedicated debit column as a debit transaction', () => {
    const [txn] = mapCsvRowsToTransactions([['2024-01-01', 'Rent', '1500']], ['date', 'description', 'debit'], 'acc-1');
    expect(txn.type).toBe('DEBIT');
    expect(txn.amount).toBe(1500);
  });

  it('uses a dedicated credit column as a credit transaction', () => {
    const [txn] = mapCsvRowsToTransactions([['2024-01-01', 'Paycheck', '1500']], ['date', 'description', 'credit'], 'acc-1');
    expect(txn.type).toBe('CREDIT');
    expect(txn.amount).toBe(1500);
  });

  it('honors a type column with CREDIT/DEBIT values', () => {
    const mapping = ['date', 'description', 'type', 'amount'];
    const [credit] = mapCsvRowsToTransactions([['2024-01-01', 'Paycheck', 'CREDIT', '1500']], mapping, 'acc-1');
    expect(credit.type).toBe('CREDIT');
    expect(credit.amount).toBe(1500);

    const [debit] = mapCsvRowsToTransactions([['2024-01-01', 'Coffee', 'DEBIT', '4.5']], mapping, 'acc-1');
    expect(debit.type).toBe('DEBIT');
    expect(debit.amount).toBe(4.5);
  });

  it('prefers the populated debit/credit column over a signed amount column', () => {
    const mapping = ['date', 'description', 'debit', 'credit'];
    const [debit] = mapCsvRowsToTransactions([['2024-01-01', 'Rent', '1500', '']], mapping, 'acc-1');
    expect(debit.type).toBe('DEBIT');
    expect(debit.amount).toBe(1500);

    const [credit] = mapCsvRowsToTransactions([['2024-01-01', 'Paycheck', '', '1500']], mapping, 'acc-1');
    expect(credit.type).toBe('CREDIT');
    expect(credit.amount).toBe(1500);
  });

  it('parses DD/MM/YYYY dates as day-first', () => {
    const mapping = ['date', 'description', 'amount'];
    const [txn] = mapCsvRowsToTransactions([['31/12/2026', 'Rent', '1500']], mapping, 'acc-1');
    const d = new Date(txn.occurredAt.seconds * 1000);
    expect(d.getDate()).toBe(31);
    expect(d.getMonth()).toBe(11);
    expect(d.getFullYear()).toBe(2026);
  });
});
