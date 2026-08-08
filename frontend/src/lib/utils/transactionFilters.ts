import { formatCurrency } from './format';

export interface TransactionFilters {
  dateFrom: string;
  dateTo: string;
  minAmount: string;
  maxAmount: string;
  categoryId: string;
  names: string[];
  type: '' | 'CREDIT' | 'DEBIT';
}

export function emptyFilters(): TransactionFilters {
  return { dateFrom: '', dateTo: '', minAmount: '', maxAmount: '', categoryId: '', names: [], type: '' };
}

export function hasActiveFilters(filters: TransactionFilters): boolean {
  return !!(filters.dateFrom || filters.dateTo || filters.minAmount || filters.maxAmount || filters.categoryId || filters.names.length || filters.type);
}

export interface FilterBubble {
  key: keyof TransactionFilters;
  value: string;
  label: string;
}

export function buildFilterBubbles(filters: TransactionFilters, categoryList: any[]): FilterBubble[] {
  const bubbles: FilterBubble[] = [];
  filters.names.forEach((n) => bubbles.push({ key: 'names', value: n, label: `Name: "${n}"` }));
  if (filters.categoryId) {
    const cat = categoryList.find((c: any) => c.id === filters.categoryId);
    bubbles.push({ key: 'categoryId', value: filters.categoryId, label: `Category: ${cat?.name || filters.categoryId}` });
  }
  if (filters.type) bubbles.push({ key: 'type', value: filters.type, label: `Type: ${filters.type === 'CREDIT' ? 'Credit' : 'Debit'}` });
  if (filters.dateFrom) bubbles.push({ key: 'dateFrom', value: filters.dateFrom, label: `From: ${filters.dateFrom}` });
  if (filters.dateTo) bubbles.push({ key: 'dateTo', value: filters.dateTo, label: `To: ${filters.dateTo}` });
  if (filters.minAmount) bubbles.push({ key: 'minAmount', value: filters.minAmount, label: `Min: ${formatCurrency(Number(filters.minAmount))}` });
  if (filters.maxAmount) bubbles.push({ key: 'maxAmount', value: filters.maxAmount, label: `Max: ${formatCurrency(Number(filters.maxAmount))}` });
  return bubbles;
}

export function removeFilterBubble(filters: TransactionFilters, bubble: FilterBubble): TransactionFilters {
  if (bubble.key === 'names') return { ...filters, names: filters.names.filter((n) => n !== bubble.value) };
  return { ...filters, [bubble.key]: '' };
}
