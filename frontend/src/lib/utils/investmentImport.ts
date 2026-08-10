import { roundMoney } from '@/lib/utils/money';
import { buildDate } from '@/lib/utils/csv';

export type InvestmentField = 'ignore' | 'symbol' | 'name' | 'type' | 'side' | 'quantity' | 'price' | 'date';

const KEYWORD_GROUPS: [InvestmentField, string[]][] = [
  ['symbol', ['symbol', 'ticker', 'code', 'isin']],
  ['name', ['name', 'scheme', 'fund', 'stock', 'company']],
  ['quantity', ['units', 'quantity', 'qty', 'unit']],
  ['price', ['price', 'nav', 'rate', 'amount', 'amt', 'value']],
  ['side', ['side', 'buy', 'sell', 'transaction type', 'debit', 'credit']],
  ['date', ['date', 'trade date', 'transaction date']]
];

// locateInvestmentTable finds the holdings/transactions table inside a broker
// export. Broker sheets often carry junk above and below; the table starts at
// the header row (≥2 keyword groups in one row) and ends at the first non-data
// row (blank, or fewer than 2 populated cells — footers the CSV parser keeps).
export function locateInvestmentTable(rows: string[][]): { headers: string[]; data: string[][] } | null {
  for (let i = 0; i < rows.length; i++) {
    const header = rows[i].map((c) => c.trim());
    let matched = 0;
    for (const [, kws] of KEYWORD_GROUPS) {
      if (header.some((c) => kws.some((k) => c.toLowerCase().includes(k)))) matched++;
    }
    if (matched < 2) continue;
    const data: string[][] = [];
    for (let j = i + 1; j < rows.length; j++) {
      const r = rows[j];
      // A data row has ≥2 populated cells. Stops at blank rows (0 cells) and
      // single-cell footers/junk that the CSV parser may have kept.
      if (r.filter((c) => c.trim() !== '').length < 2) break;
      data.push(r);
    }
    if (data.length === 0) continue;
    return { headers: header, data };
  }
  return null;
}

export function guessInvestmentMapping(header: string): InvestmentField {
  const l = header.toLowerCase();
  for (const [field, kws] of KEYWORD_GROUPS) {
    if (kws.some((k) => l.includes(k))) return field;
  }
  return 'ignore';
}

export interface ImportInvestmentRow {
  symbol: string;
  name: string;
  investmentType: 'STOCK' | 'MUTUAL_FUND';
  side: number;
  quantity: number;
  price: number;
  occurredAt: { seconds: number; nanos: number };
  externalId: string;
}

function resolveType(cell: string | undefined): ImportInvestmentRow['investmentType'] {
  const s = (cell || '').toLowerCase();
  if (/(mutual|fund)/.test(s)) return 'MUTUAL_FUND';
  return 'STOCK';
}

function resolveSide(cell: string | undefined): number {
  const s = (cell || '').toLowerCase();
  if (/(sell|debit|redeem|-)/.test(s)) return -1;
  return 1;
}

export function mapImportRows(rows: string[][], mapping: string[], fileKey: string): ImportInvestmentRow[] {
  const idx = (f: InvestmentField) => mapping.indexOf(f);
  return rows
    .map((row, i) => {
      const symbol = (idx('symbol') >= 0 ? row[idx('symbol')] : '').trim();
      const name = idx('name') >= 0 && row[idx('name')] ? row[idx('name')].trim() : symbol;
      const quantity = parseFloat(row[idx('quantity')]) || 0;
      return {
        symbol,
        name,
        investmentType: resolveType(idx('type') >= 0 ? row[idx('type')] : undefined),
        side: resolveSide(idx('side') >= 0 ? row[idx('side')] : undefined),
        quantity: roundMoney(quantity),
        price: roundMoney(parseFloat(row[idx('price')]) || 0),
        occurredAt: { seconds: Math.floor(buildDate(idx('date') >= 0 ? row[idx('date')] : '') / 1000), nanos: 0 },
        externalId: fileKey ? `${fileKey}:${i}` : ''
      };
    })
    .filter((r) => r.symbol && r.quantity > 0);
}
