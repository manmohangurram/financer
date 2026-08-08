import { describe, it, expect } from 'vitest';
import { emptyFilters, hasActiveFilters, buildActiveFilterChips, clearFilterKey } from '@/lib/utils/transactionFilters';

describe('transactionFilters', () => {
  it('emptyFilters defaults nameMatch to contains and type to empty', () => {
    const f = emptyFilters();
    expect(f).toEqual({ dateFrom: '', dateTo: '', minAmount: '', maxAmount: '', categoryId: '', name: '', nameMatch: 'contains', type: '' });
  });

  it('hasActiveFilters is true when type is set', () => {
    expect(hasActiveFilters({ ...emptyFilters(), type: 'CREDIT' })).toBe(true);
  });

  it('buildActiveFilterChips includes name, amount (currency-formatted), category, and type chips', () => {
    const f = { ...emptyFilters(), name: 'uber', minAmount: '50', maxAmount: '100', categoryId: 'cat1', type: 'DEBIT' };
    const chips = buildActiveFilterChips(f, [{ id: 'cat1', name: 'Transportation' }]);
    expect(chips).toEqual([
      { key: 'minAmount', label: 'Min: ₹50.00' },
      { key: 'maxAmount', label: 'Max: ₹100.00' },
      { key: 'categoryId', label: 'Category: Transportation' },
      { key: 'name', label: 'Name: "uber"' },
      { key: 'type', label: 'Type: Debit' }
    ]);
  });

  it('clearFilterKey resets a key to empty string', () => {
    const f = { ...emptyFilters(), name: 'x', type: 'CREDIT' };
    expect(clearFilterKey(f, 'name')).toMatchObject({ name: '', type: 'CREDIT' });
    expect(clearFilterKey(f, 'type')).toMatchObject({ type: '' });
  });
});
