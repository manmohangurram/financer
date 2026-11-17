import { describe, expect, it } from 'vitest';
import { parseStatementText } from './statementPdf';

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
