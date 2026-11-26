import { describe, expect, it } from 'vitest';
import { budgetColor, budgetTone } from './budget';

describe('budgetTone', () => {
  it('stays calm below 70%', () => {
    expect(budgetTone(0)).toBe('ok');
    expect(budgetTone(69.9)).toBe('ok');
  });

  it('warns from 70% up to 89%', () => {
    expect(budgetTone(70)).toBe('warn');
    expect(budgetTone(85)).toBe('warn');
    expect(budgetTone(89.9)).toBe('warn');
  });

  it('is hot from 90% and stays hot when over', () => {
    expect(budgetTone(90)).toBe('hot');
    expect(budgetTone(100)).toBe('hot');
    expect(budgetTone(180)).toBe('hot');
  });

  it('resolves to the token every bar and ring draws with', () => {
    expect(budgetColor(12)).toBe('var(--accent)');
    expect(budgetColor(72)).toBe('var(--stamp)');
    expect(budgetColor(92)).toBe('var(--debit)');
  });
});
