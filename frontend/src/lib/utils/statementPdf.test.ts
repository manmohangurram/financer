import { describe, expect, it } from 'vitest';
import { parseStatement, parseStatementText } from './statementPdf';

const STATEMENT = `--- PAGE 1 ---
Page 1 of 1
Date Narration Chq. / Ref No. Value Date Withdrawal Amount Deposit Amount Closing Balance*
NEFT CR-ICIC0SF0002-MDH WHEELS PRIVATE L
24/08/2021 24825922781DC 24/08/2021 0.00 4,000.00 4,000.00
-G MANMOHAN-24825922781DC
26/08/2021 POS 435584XXXXXX6701 KFC SAPPHIRE COR 000000141026 26/08/2021 599.00 0.00 3,401.00
UPI-SITARA GUESTLINE-PAYTM-69168809@PAYTM
27/08/2021 M-PYTM0123456-221542332667-OID 221542332667 27/08/2021 100.00 0.00 3,301.00
23200`;

describe('parseStatementText', () => {
  it('extracts rows and validates the running balance', () => {
    const out = parseStatementText(STATEMENT);
    expect(out.rows).toHaveLength(3);
    expect(out.opening).toBe(0);
    expect(out.closing).toBe(3301);
    expect(out.credits).toBe(4000);
    expect(out.debits).toBe(699);
    expect(out.valid).toBe(true);
    expect(out.problem).toBeUndefined();

    expect(out.rows[0]).toMatchObject({ amount: 4000, type: 'CREDIT', occurredAt: '2021-08-24' });
    expect(out.rows[1]).toMatchObject({ amount: 599, type: 'DEBIT', occurredAt: '2021-08-26', name: 'KFC SAPPHIRE COR' });
    expect(out.rows[2]).toMatchObject({ amount: 100, type: 'DEBIT', occurredAt: '2021-08-27', name: 'SITARA GUESTLINE' });
  });

  it('flags a statement whose balances do not add up', () => {
    const broken = STATEMENT.replace('3,401.00', '3,900.00');
    const out = parseStatementText(broken);
    expect(out.valid).toBe(false);
    expect(out.problem).toMatch(/Balance breaks|do not add up/);
  });

  it('handles a negative running balance from an EMI reversal', () => {
    const text = `--- PAGE 1 ---
24/08/2021 NEFT CR-X-MDH 1 24/08/2021 0.00 1,000.00 1,000.00
25/08/2021 DC EMI LOAN# 1 25/08/2021 25/08/2021 5,000.00 0.00 -4,000.00
25/08/2021 DC EMI LOAN# 1 25/08/2021 25/08/2021 0.00 5,000.00 1,000.00`;
    const out = parseStatementText(text);
    expect(out.rows).toHaveLength(3);
    expect(out.valid).toBe(true);
    expect(out.rows[1]).toMatchObject({ amount: 5000, type: 'DEBIT' });
  });
});

describe('statement column layouts', () => {
  it('reads amount + Dr/Cr + balance', () => {
    const text = `--- PAGE 1 ---
01/04/2022 NARR ONE 01/04/2022 1,000.00 Dr 9,000.00
02/04/2022 NARR TWO 02/04/2022 5,000.00 Cr 14,000.00`;
    const out = parseStatementText(text);
    expect(out.layout).toBe('amount-marker');
    expect(out.rows).toHaveLength(2);
    expect(out.rows[0]).toMatchObject({ amount: 1000, type: 'DEBIT', balanceAfter: 9000 });
    expect(out.rows[1]).toMatchObject({ amount: 5000, type: 'CREDIT', balanceAfter: 14000 });
    expect(out.valid).toBe(true);
  });

  it('reads a signed amount + balance and infers the rest from the balance', () => {
    const text = `--- PAGE 1 ---
01/04/2022 NARR ONE 01/04/2022 -1,000.00 9,000.00
02/04/2022 NARR TWO 02/04/2022 5,000.00 14,000.00`;
    const out = parseStatementText(text);
    expect(out.layout).toBe('amount-delta');
    expect(out.rows[0]).toMatchObject({ amount: 1000, type: 'DEBIT' });
    expect(out.rows[1]).toMatchObject({ amount: 5000, type: 'CREDIT' });
    expect(out.valid).toBe(true);
  });

  it('accepts a forced layout and amount-only rows', () => {
    const text = `--- PAGE 1 ---
01/04/2022 NARR ONE 01/04/2022 1,000.00 Dr`;
    const out = parseStatementText(text, 'amount-only');
    expect(out.layout).toBe('amount-only');
    expect(out.rows).toHaveLength(1);
    expect(out.rows[0]).toMatchObject({ amount: 1000, type: 'DEBIT' });
    expect(out.valid).toBe(true);
  });
});

describe('positioned columns (new HDFC layout)', () => {
  const mk = (x: number, s: string) => ({ x, s });
  const lines = [
    { text: 'Date Narration Chq./Ref.No. Value Dt Withdrawal Amt. Deposit Amt. Closing Balance', items: [
      mk(40, 'Date'), mk(144, 'Narration'), mk(284, 'Chq./Ref.No.'), mk(362, 'Value Dt'),
      mk(405, 'Withdrawal Amt.'), mk(491, 'Deposit Amt.'), mk(564, 'Closing Balance')
    ] },
    { text: '01/04/26 UPI-A 0000316604154309 01/04/26 50,000.00 232,229.71', items: [
      mk(34, '01/04/26'), mk(72, 'UPI-A'), mk(289, '0000316604154309'), mk(362, '01/04/26'),
      mk(438, '50,000.00'), mk(591, '232,229.71')
    ] },
    { text: '01/04/26 UPI-B 0000064559085757 01/04/26 1,591.00 233,820.71', items: [
      mk(34, '01/04/26'), mk(72, 'UPI-B'), mk(289, '0000064559085757'), mk(362, '01/04/26'),
      mk(520, '1,591.00'), mk(591, '233,820.71')
    ] }
  ];

  it('uses the header x positions to tell debit from credit', () => {
    const out = parseStatement(lines);
    expect(out.rows).toHaveLength(2);
    expect(out.rows[0]).toMatchObject({ amount: 50000, type: 'DEBIT', occurredAt: '2026-04-01' });
    expect(out.rows[1]).toMatchObject({ amount: 1591, type: 'CREDIT' });
    expect(out.layout).toBe('columns');
    expect(out.valid).toBe(true);
  });
});

describe('name cleaning', () => {
  it('drops long reference numbers between name parts', () => {
    const text = `--- PAGE 1 ---
01/04/26 UPI-GURRAM 0000316604154309 SWAPNA-PMADAN113@AXL-SBIN001-316604154309 01/04/26 50,000.00 232,229.71`;
    const out = parseStatementText(text);
    expect(out.rows[0].name).toBe('GURRAM SWAPNA');
  });
});
