import { formatCurrency } from './format';

export interface TransactionFilters {
  dateFrom: string;
  dateTo: string;
  minAmount: string;
  maxAmount: string;
  categoryId: string;
  name: string;
  type: '' | 'CREDIT' | 'DEBIT';
}

export function emptyFilters(): TransactionFilters {
  return { dateFrom: '', dateTo: '', minAmount: '', maxAmount: '', categoryId: '', name: '', type: '' };
}

export function hasActiveFilters(filters: TransactionFilters): boolean {
  return !!(filters.dateFrom || filters.dateTo || filters.minAmount || filters.maxAmount || filters.categoryId || filters.name || filters.type);
}

export interface FilterChip {
  key: keyof TransactionFilters;
  label: string;
}

export function buildActiveFilterChips(filters: TransactionFilters, categoryList: any[]): FilterChip[] {
  const chips: FilterChip[] = [];
  if (filters.dateFrom) chips.push({ key: 'dateFrom', label: `From: ${filters.dateFrom}` });
  if (filters.dateTo) chips.push({ key: 'dateTo', label: `To: ${filters.dateTo}` });
  if (filters.minAmount) chips.push({ key: 'minAmount', label: `Min: ${formatCurrency(Number(filters.minAmount))}` });
  if (filters.maxAmount) chips.push({ key: 'maxAmount', label: `Max: ${formatCurrency(Number(filters.maxAmount))}` });
  if (filters.categoryId) {
    const cat = categoryList.find((c: any) => c.id === filters.categoryId);
    chips.push({ key: 'categoryId', label: `Category: ${cat?.name || filters.categoryId}` });
  }
  if (filters.name) chips.push({ key: 'name', label: `Name: "${filters.name}"` });
  if (filters.type) chips.push({ key: 'type', label: `Type: ${filters.type === 'CREDIT' ? 'Credit' : 'Debit'}` });
  return chips;
}

export function clearFilterKey(filters: TransactionFilters, key: keyof TransactionFilters): TransactionFilters {
  return { ...filters, [key]: '' };
}
