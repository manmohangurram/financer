import { describe, it, expect, vi } from 'vitest';
import { CATEGORY_PALETTE, CATEGORY_PALETTE_LIGHT, categoryColorMap } from '@/lib/utils/categoryColor';

const theme = vi.hoisted(() => ({ value: 'dark' }));
vi.mock('@/lib/theme', () => ({ resolved: theme }));

const HEX = /^#[0-9a-f]{6}$/i;

describe('categoryColor', () => {
  it('returns hex colors for every category, including past the 16-color palettes', () => {
    const names = Array.from({ length: 20 }, (_, i) => `Category ${i}`);
    const map = categoryColorMap(names);
    expect(Object.keys(map)).toHaveLength(names.length);
    for (const c of Object.values(map)) expect(c).toMatch(HEX);
  });

  it('wraps past the 16-color palette, every value stays a valid palette hex', () => {
    const names = Array.from({ length: 20 }, (_, i) => `Category ${i}`);
    const map = categoryColorMap(names);
    const palette = new Set(CATEGORY_PALETTE);
    for (const c of Object.values(map)) expect(palette.has(c)).toBe(true);
  });

  it('uses the darker light-theme palette in light mode', () => {
    theme.value = 'light';
    const map = categoryColorMap(['Food', 'Transport']);
    theme.value = 'dark';
    const sortedNames = ['Food', 'Transport'].sort();
    expect(Object.values(map)).toEqual(sortedNames.map((_, i) => CATEGORY_PALETTE_LIGHT[i]));
    expect(Object.values(map).every((c) => c.match(HEX))).toBe(true);
  });
});
