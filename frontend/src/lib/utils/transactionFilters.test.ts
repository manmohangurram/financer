import { describe, it, expect } from 'vitest';
import { emptyFilters, hasActiveFilters, buildFilterBubbles, removeFilterBubble, type TransactionFilters } from '@/lib/utils/transactionFilters';

describe('transactionFilters', () => {
  it('emptyFilters defaults names to an empty array and type to empty', () => {
    const f = emptyFilters();
    expect(f).toEqual({ dateFrom: '', dateTo: '', minAmount: '', maxAmount: '', categoryId: '', names: [], type: '' });
  });

  it('hasActiveFilters is true when names or type are set', () => {
    expect(hasActiveFilters({ ...emptyFilters(), names: ['uber'] })).toBe(true);
    expect(hasActiveFilters({ ...emptyFilters(), type: 'CREDIT' })).toBe(true);
  });

  it('buildFilterBubbles renders name, category, type, date, and currency-formatted amount bubbles', () => {
    const f: TransactionFilters = {
      ...emptyFilters(),
      names: ['uber', 'netflix'],
      categoryId: 'cat1',
      type: 'DEBIT',
      dateFrom: '2026-08-01',
      dateTo: '2026-08-31',
      minAmount: '50',
      maxAmount: '100'
    };
    const bubbles = buildFilterBubbles(f, [{ id: 'cat1', name: 'Transportation' }]);
    expect(bubbles).toEqual([
      { key: 'names', value: 'uber', label: 'Name: "uber"' },
      { key: 'names', value: 'netflix', label: 'Name: "netflix"' },
      { key: 'categoryId', value: 'cat1', label: 'Category: Transportation' },
      { key: 'type', value: 'DEBIT', label: 'Type: Debit' },
      { key: 'dateFrom', value: '2026-08-01', label: 'From: 2026-08-01' },
      { key: 'dateTo', value: '2026-08-31', label: 'To: 2026-08-31' },
      { key: 'minAmount', value: '50', label: 'Min: ₹50.00' },
      { key: 'maxAmount', value: '100', label: 'Max: ₹100.00' }
    ]);
  });

  it('removeFilterBubble removes one name but keeps the rest', () => {
    const f: TransactionFilters = { ...emptyFilters(), names: ['uber', 'netflix'], type: 'DEBIT' };
    const f2 = removeFilterBubble(f, { key: 'names', value: 'uber', label: '' });
    expect(f2.names).toEqual(['netflix']);
    expect(f2.type).toBe('DEBIT');
  });

  it('removeFilterBubble clears a single-value filter', () => {
    const f: TransactionFilters = { ...emptyFilters(), type: 'CREDIT', categoryId: 'cat1' };
    expect(removeFilterBubble(f, { key: 'type', value: 'CREDIT', label: '' }).type).toBe('');
  });
});
