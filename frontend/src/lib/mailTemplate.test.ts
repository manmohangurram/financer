import { describe, expect, it } from 'vitest';
import { cleanedTemplate, emptyTemplate, templateProblem } from './mailTemplate';

describe('mail template helpers', () => {
  it('starts with one empty pattern per field', () => {
    const t = emptyTemplate();
    expect(Object.keys(t)).toEqual(['amount', 'debit', 'credit', 'account', 'merchant', 'date']);
    expect(t.amount).toEqual(['']);
  });

  it('drops blank and whitespace-only rows', () => {
    const t = emptyTemplate();
    t.amount = ['  Rs\\.?\\s*(?P<amount>[\\d,]+)  ', '', '   '];
    expect(cleanedTemplate(t).amount).toEqual(['Rs\\.?\\s*(?P<amount>[\\d,]+)']);
  });

  it('requires amount, date and a direction', () => {
    const t = emptyTemplate();
    expect(templateProblem(t)).toBe('Add an amount pattern.');
    t.amount = ['a'];
    expect(templateProblem(t)).toBe('Add a date pattern.');
    t.date = ['d'];
    expect(templateProblem(t)).toBe('Add a debit or credit pattern.');
    t.debit = ['b'];
    expect(templateProblem(t)).toBeNull();
    t.debit = [];
    t.credit = ['c'];
    expect(templateProblem(t)).toBeNull();
  });
});
