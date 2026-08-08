// 4 theme-matching families (blue, emerald, violet, red) x 4 dark shades each, interleaved so adjacent categories differ in hue.
export const CATEGORY_PALETTE = [
  '#3b82f6', '#10b981', '#8b5cf6', '#ef4444',
  '#2563eb', '#059669', '#7c3aed', '#dc2626',
  '#1d4ed8', '#047857', '#6d28d9', '#b91c1c',
  '#1e40af', '#065f46', '#5b21b6', '#991b1b'
];

export function categoryColorMap(names: string[]): Record<string, string> {
  const sorted = Array.from(new Set(names)).sort();
  const map: Record<string, string> = {};
  sorted.forEach((n, i) => {
    const hue = (i * 137.508) % 360;
    const fixed = hue >= 45 && hue <= 70 ? (hue + 180) % 360 : hue; // skip yellow band
    map[n] = i < CATEGORY_PALETTE.length ? CATEGORY_PALETTE[i] : `hsl(${fixed} 70% 50%)`;
  });
  return map;
}
