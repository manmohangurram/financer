// Header → field guessing for the investments import mapping UI. Locating the
// table and mapping rows now happen on the backend (`src/import/investment.rs`).

export type InvestmentField = 'ignore' | 'symbol' | 'name' | 'type' | 'side' | 'quantity' | 'price' | 'date';

const KEYWORD_GROUPS: [InvestmentField, string[]][] = [
  ['symbol', ['symbol', 'ticker', 'code', 'isin']],
  ['name', ['name', 'scheme', 'fund', 'stock', 'company']],
  ['quantity', ['units', 'quantity', 'qty', 'unit']],
  ['price', ['price', 'nav', 'rate', 'amount', 'amt', 'value']],
  ['side', ['side', 'buy', 'sell', 'transaction type', 'debit', 'credit']],
  ['date', ['date', 'trade date', 'transaction date']]
];

export function guessInvestmentMapping(header: string): InvestmentField {
  const l = header.toLowerCase();
  for (const [field, kws] of KEYWORD_GROUPS) {
    if (kws.some((k) => l.includes(k))) return field;
  }
  return 'ignore';
}
