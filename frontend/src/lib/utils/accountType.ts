export interface AccountTypeOption {
  value: string;
  name: string;
}

export const ACCOUNT_TYPE_OPTIONS: AccountTypeOption[] = [
  { value: 'ACCOUNT_TYPE_CHECKING', name: 'Checking' },
  { value: 'ACCOUNT_TYPE_SAVINGS', name: 'Savings' },
  { value: 'ACCOUNT_TYPE_CREDIT_CARD', name: 'Credit Card' },
  { value: 'ACCOUNT_TYPE_LOAN', name: 'Loan' },
  { value: 'ACCOUNT_TYPE_CRYPTO_WALLET', name: 'Crypto Wallet' }
];

const LABELS: Record<string, string> = Object.fromEntries(ACCOUNT_TYPE_OPTIONS.map((o) => [o.value, o.name]));
const ICONS: Record<string, string> = {
  ACCOUNT_TYPE_CHECKING: '🏦',
  ACCOUNT_TYPE_SAVINGS: '💰',
  ACCOUNT_TYPE_CREDIT_CARD: '💳',
  ACCOUNT_TYPE_LOAN: '🏷️',
  ACCOUNT_TYPE_CRYPTO_WALLET: '🪙'
};

export function accountTypeLabel(type: string): string {
  return LABELS[type] || 'Checking';
}

export function accountTypeIcon(type: string): string {
  return ICONS[type] || '🏦';
}

export function isCredit(type: string): boolean {
  return type === 'ACCOUNT_TYPE_CREDIT_CARD';
}

export function isDebt(type: string): boolean {
  return type === 'ACCOUNT_TYPE_LOAN' || type === 'ACCOUNT_TYPE_CREDIT_CARD';
}