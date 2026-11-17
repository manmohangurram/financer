// Manual extraction templates: several regexes per field, first match wins.

export interface TemplatePatterns {
  amount: string[];
  debit: string[];
  credit: string[];
  account: string[];
  merchant: string[];
  date: string[];
}

export type TemplatePatternKey = keyof TemplatePatterns;

/** Storage-level fields; each one is a list of patterns. */
export const TEMPLATE_FIELDS: Array<{ key: TemplatePatternKey; label: string }> = [
  { key: 'amount', label: 'Amount' },
  { key: 'debit', label: 'Debit wording' },
  { key: 'credit', label: 'Credit wording' },
  { key: 'account', label: 'Account' },
  { key: 'merchant', label: 'Merchant' },
  { key: 'date', label: 'Date' }
];

/** Editor fields; Direction owns the debit and credit lists together. */
export interface TemplatePart {
  key: TemplatePatternKey;
  label?: string;
  hint?: string;
}

export interface TemplateFieldView {
  key: string;
  label: string;
  required?: boolean;
  hint?: string;
  parts: TemplatePart[];
}

export const FIELD_VIEWS: TemplateFieldView[] = [
  {
    key: 'amount',
    label: 'Amount',
    required: true,
    hint: 'Capture the number only. Currency symbols and codes are ignored, so both Rs.2,050.00 and INR 2050 work.',
    parts: [{ key: 'amount' }]
  },
  {
    key: 'direction',
    label: 'Direction',
    required: true,
    hint: 'Money out matches debit, money in matches credit. They must never both match one mail.',
    parts: [
      { key: 'debit', label: 'Debit wording', hint: 'e.g. debited, spent on your card' },
      { key: 'credit', label: 'Credit wording', hint: 'e.g. credited, received' }
    ]
  },
  {
    key: 'date',
    label: 'Date',
    required: true,
    hint: 'Capture the date phrase. 2-digit years are accepted and normalised.',
    parts: [{ key: 'date' }]
  },
  {
    key: 'account',
    label: 'Account',
    hint: 'Capture the last four digits; the mask in front of them is optional.',
    parts: [{ key: 'account' }]
  },
  {
    key: 'merchant',
    label: 'Merchant',
    hint: 'Capture the payee. Bound it with the literal that follows so it cannot run on.',
    parts: [{ key: 'merchant' }]
  }
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
