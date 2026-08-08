import { describe, expect, it } from 'vitest';
import { roundMoney } from './money';

describe('roundMoney', () => {
  it('rounds to 2 decimals', () => {
    expect(roundMoney(1.234)).toBe(1.23);
    expect(roundMoney(1.235)).toBe(1.24);
    expect(roundMoney(0)).toBe(0);
  });

  it('handles floating-point edge cases the naive formula misses', () => {
    expect(roundMoney(1.005)).toBe(1.01);
    expect(roundMoney(2.675)).toBe(2.68);
    expect(roundMoney(15.99 + 0.01)).toBe(16);
  });
});
