import { resolved } from '@/lib/theme';

// 16 light hues that all pass WCAG AA (>=4.5:1) as text on the dark surfaces,
// interleaved so adjacent categories differ in hue.
export const CATEGORY_PALETTE = [
  '#60a5fa', '#34d399', '#a78bfa', '#f87171',
  '#93c5fd', '#6ee7b7', '#c4b5fd', '#fca5a5',
  '#38bdf8', '#a7f3d0', '#f0abfc', '#fbbf24',
  '#818cf8', '#10b981', '#d8b4fe', '#fdba74'
];

// Darker hues that pass AA (>=4.5:1) as text on the light surfaces and on the
// 9.4%-tinted badge backgrounds (base-200).
export const CATEGORY_PALETTE_LIGHT = [
  '#1d4ed8', '#166534', '#6d28d9', '#991b1b',
  '#1e40af', '#065f46', '#5b21b6', '#7f1d1d',
  '#0369a1', '#115e59', '#a21caf', '#92400e',
  '#4338ca', '#14532d', '#86198f', '#9a3412'
];

export function categoryColorMap(names: string[]): Record<string, string> {
  const palette = resolved.value === 'light' ? CATEGORY_PALETTE_LIGHT : CATEGORY_PALETTE;
  const sorted = Array.from(new Set(names)).sort();
  const map: Record<string, string> = {};
  sorted.forEach((n, i) => {
    map[n] = palette[i % palette.length];
  });
  return map;
}
