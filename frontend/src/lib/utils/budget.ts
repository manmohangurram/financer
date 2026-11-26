/**
 * Budget progress has three states, everywhere in the app: comfortable, close to
 * the limit, and at the limit. The colour is a signal, so the numbers that carry
 * it are defined once — a bar on the dashboard and a bar on Spending must never
 * disagree about what "90%" looks like.
 */
export type BudgetTone = 'ok' | 'warn' | 'hot';

export const WARN_AT = 70;
export const HOT_AT = 90;

export function budgetTone(usedPercent: number): BudgetTone {
  if (usedPercent >= HOT_AT) return 'hot';
  if (usedPercent >= WARN_AT) return 'warn';
  return 'ok';
}

/** A CSS colour for SVG strokes and inline styles (dashboard ring, bars). */
export function budgetColor(usedPercent: number): string {
  return { ok: 'var(--accent)', warn: 'var(--stamp)', hot: 'var(--debit)' }[budgetTone(usedPercent)];
}
