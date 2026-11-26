import { reactive } from 'vue';

/** The chart palette, read from the same CSS tokens the rest of the UI uses.
 *  Apex needs concrete colour strings, so they are resolved once per theme. */
export interface ChartPalette {
  accent: string;
  accentSoft: string;
  ink: string;
  inkMute: string;
  faint: string;
  rule: string;
  page: string;
  credit: string;
  debit: string;
  ramp: string[];
}

function token(name: string, fallback: string): string {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}

function isDark(): boolean {
  return document.documentElement.dataset.theme === 'dark';
}

export function readChartPalette(): ChartPalette {
  const dark = isDark();
  return {
    accent: token('--accent', dark ? '#7dd3fc' : '#0369a1'),
    accentSoft: token('--accent-wash', dark ? 'rgba(125,211,252,.12)' : '#e0f2fe'),
    ink: token('--ink', dark ? '#e9eef6' : '#0f1620'),
    inkMute: token('--ink-mute', dark ? '#8b99ab' : '#6b7891'),
    faint: token('--ink-faint', dark ? '#5f6d80' : '#98a3b6'),
    rule: token('--rule-soft', dark ? 'rgba(255,255,255,.05)' : 'rgba(15,22,32,.06)'),
    page: token('--page', dark ? '#111823' : '#ffffff'),
    credit: token('--credit', dark ? '#4ade80' : '#047857'),
    debit: token('--debit', dark ? '#fb7185' : '#be123c'),
    // rank-ordered ramp: biggest share first, grey tail for "Others"
    ramp: [0, 1, 2, 3, 4, 5, 6, 7].map((i) => token(`--chart-${i}`, dark ? '#7dd3fc' : '#0369a1'))
  };
}

const palette = reactive<ChartPalette>(readChartPalette());

// Re-read the tokens when the theme flips (data-theme lives on <html>).
if (typeof window !== 'undefined') {
  new MutationObserver(() => Object.assign(palette, readChartPalette())).observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-theme']
  });
}

export { palette };
