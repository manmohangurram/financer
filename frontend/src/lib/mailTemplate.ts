// Manual extraction templates: several regexes per field, first match wins.

export interface TemplatePatterns {
  amount: string[];
  debit: string[];
  credit: string[];
  account: string[];
  merchant: string[];
  date: string[];
}

export type TemplateFieldKey = keyof TemplatePatterns;

export interface TemplateField {
  key: TemplateFieldKey;
  label: string;
  required?: boolean;
  hint?: string;
}

export const TEMPLATE_FIELDS: TemplateField[] = [
  { key: 'amount', label: 'Amount', required: true, hint: 'Capture the number only — currency symbols and codes are ignored.' },
  { key: 'debit', label: 'Debit wording', hint: 'A match means money out.' },
  { key: 'credit', label: 'Credit wording', hint: 'A match means money in.' },
  { key: 'account', label: 'Account ending', hint: 'Capture the last four digits.' },
  { key: 'merchant', label: 'Merchant / payee' },
  { key: 'date', label: 'Date', required: true, hint: '2-digit years are accepted.' }
];

export function emptyTemplate(): TemplatePatterns {
  return { amount: [''], debit: [''], credit: [''], account: [''], merchant: [''], date: [''] };
}

/** Drops blank rows so the server sees only real patterns. */
export function cleanedTemplate(patterns: TemplatePatterns): TemplatePatterns {
  const out = {} as TemplatePatterns;
  for (const field of TEMPLATE_FIELDS) {
    out[field.key] = patterns[field.key].map((p) => p.trim()).filter(Boolean);
  }
  return out;
}

/** Missing required fields, or a missing direction pair, described for the user. */
export function templateProblem(patterns: TemplatePatterns): string | null {
  const clean = cleanedTemplate(patterns);
  if (clean.amount.length === 0) return 'Add an amount pattern.';
  if (clean.date.length === 0) return 'Add a date pattern.';
  if (clean.debit.length === 0 && clean.credit.length === 0) return 'Add a debit or credit pattern.';
  return null;
}
