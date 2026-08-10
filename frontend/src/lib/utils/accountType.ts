export interface AccountTypeOption {
  value: string;
  name: string;
}

export const ACCOUNT_TYPE_OPTIONS: AccountTypeOption[] = [
  { value: 'CURRENT', name: 'Current' },
  { value: 'SAVINGS', name: 'Savings' },
  { value: 'LOAN', name: 'Loan' },
  { value: 'CREDIT_CARD', name: 'Credit Card' }
];

const LABELS: Record<string, string> = Object.fromEntries(ACCOUNT_TYPE_OPTIONS.map((o) => [o.value, o.name]));
const ICONS: Record<string, string> = {
  CURRENT: '🏦',
  SAVINGS: '💰',
  LOAN: '🏷️',
  CREDIT_CARD: '💳'
};

export function accountTypeLabel(type: string): string {
  return LABELS[type] || 'Current';
}

export function accountTypeIcon(type: string): string {
  return ICONS[type] || '🏦';
}

export function isCredit(type: string): boolean {
  return type === 'CREDIT_CARD';
}

export function isDebt(type: string): boolean {
  return type === 'LOAN' || type === 'CREDIT_CARD';
}
