// 16 light hues that all pass WCAG AA (>=4.5:1) as text on the dark surfaces,
// interleaved so adjacent categories differ in hue.
export const CATEGORY_PALETTE = [
  '#60a5fa', '#34d399', '#a78bfa', '#f87171',
  '#93c5fd', '#6ee7b7', '#c4b5fd', '#fca5a5',
  '#38bdf8', '#a7f3d0', '#f0abfc', '#fbbf24',
  '#818cf8', '#10b981', '#d8b4fe', '#fdba74'
];

export function categoryColorMap(names: string[]): Record<string, string> {
  const sorted = Array.from(new Set(names)).sort();
  const map: Record<string, string> = {};
  sorted.forEach((n, i) => {
    const hue = (i * 137.508) % 360;
    const fixed = hue >= 45 && hue <= 70 ? (hue + 180) % 360 : hue; // skip yellow band
    map[n] = i < CATEGORY_PALETTE.length ? CATEGORY_PALETTE[i] : `hsl(${fixed} 80% 70%)`;
  });
  return map;
}
