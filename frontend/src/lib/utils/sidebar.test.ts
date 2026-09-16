import { describe, expect, it } from 'vitest';
import { resolveActiveHref, sliceAccounts, NAV_HREFS } from './sidebar';

describe('resolveActiveHref', () => {
  it('matches an exact route', () => {
    expect(resolveActiveHref('/dashboard')).toBe('/dashboard');
    expect(resolveActiveHref('/accounts')).toBe('/accounts');
  });

  it('prefers the longest match so Spending does not also light up Accounts', () => {
    expect(resolveActiveHref('/accounts/spending')).toBe('/accounts/spending');
    expect(resolveActiveHref('/accounts/categories')).toBe('/accounts/categories');
    expect(resolveActiveHref('/accounts/rules')).toBe('/accounts/rules');
  });

  it('falls back to the parent for a sub-page with no nav item', () => {
    expect(resolveActiveHref('/accounts/manage')).toBe('/accounts');
    expect(resolveActiveHref('/investments/abc-123')).toBe('/investments');
  });

  it('returns null when nothing owns the route', () => {
    expect(resolveActiveHref('/settings')).toBeNull();
    expect(resolveActiveHref('/')).toBeNull();
  });

  it('does not match on a partial path segment', () => {
    expect(resolveActiveHref('/accounts-archive')).toBeNull();
  });

  it('covers every section href exactly once', () => {
    expect(new Set(NAV_HREFS).size).toBe(NAV_HREFS.length);
    for (const href of NAV_HREFS) expect(resolveActiveHref(href)).toBe(href);
  });
});

describe('sliceAccounts', () => {
  const many = [1, 2, 3, 4, 5, 6, 7];

  it('caps the list and counts the remainder', () => {
    expect(sliceAccounts(many, 5)).toEqual({ shown: [1, 2, 3, 4, 5], more: 2 });
  });

  it('reports no remainder when everything fits', () => {
    expect(sliceAccounts([1, 2], 5)).toEqual({ shown: [1, 2], more: 0 });
    expect(sliceAccounts(many, 7)).toEqual({ shown: many, more: 0 });
  });

  it('handles an empty account list', () => {
    expect(sliceAccounts([], 5)).toEqual({ shown: [], more: 0 });
  });
});
