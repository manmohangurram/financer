// Header → field guessing for the import mapping UI. Parsing and mapping now
// happen on the backend (`src/import/`).

export type CsvField = 'ignore' | 'date' | 'description' | 'amount' | 'type' | 'debit' | 'credit';

export function guessMapping(header: string): CsvField {
  const l = header.toLowerCase();
  // A running balance is not an amount; a value date is a secondary date.
  if (l.includes('balance')) return 'ignore';
  if (l.includes('value date')) return 'ignore';
  if (l.includes('withdrawal') || l.includes('debit') || l.includes('paid out')) return 'debit';
  if (l.includes('deposit') || l.includes('credit') || l.includes('paid in')) return 'credit';
  if (l.includes('narration') || l.includes('particulars') || l.includes('details') || l.includes('remarks')) return 'description';
  if (l.includes('desc') || l.includes('name') || l.includes('memo')) return 'description';
  if (l.includes('type') || l.includes('direction')) return 'type';
  if (l.includes('date')) return 'date';
  if (l.includes('amount')) return 'amount';
  return 'ignore';
}
