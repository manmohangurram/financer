import { reactive } from 'vue';

/** The chart palette, read from the same CSS tokens the rest of the UI uses.
 *  Apex needs concrete colour strings, so they are resolved once per theme. */
export interface ChartPalette {
  accent: string;
  accentSoft: string;
  accentQuiet: string;
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

export function readChartPalette(): ChartPalette {
  return {
    accent: token('--accent', 'oklch(0.455 0.075 205)'),
    accentSoft: token('--accent-wash', 'oklch(0.94 0.02 205)'),
    // accent at 28% — the receded state when another day is picked
    accentQuiet: 'oklch(0.455 0.075 205 / 0.28)',
    ink: token('--ink', 'oklch(0.255 0.02 245)'),
    inkMute: token('--ink-mute', 'oklch(0.585 0.014 245)'),
    faint: token('--ink-faint', 'oklch(0.7 0.01 245)'),
    rule: token('--rule-soft', 'oklch(0 0 0 / 0.07)'),
    page: token('--page', 'oklch(0.995 0.003 85)'),
    credit: token('--credit', 'oklch(0.505 0.1 155)'),
    debit: token('--debit', 'oklch(0.52 0.15 28)'),
    // rank-ordered ink ramp: darkest = biggest share, grey tail for "Others"
    ramp: [
      token('--accent', 'oklch(0.36 0.045 205)'),
      'oklch(0.44 0.045 205)',
      'oklch(0.52 0.04 205)',
      'oklch(0.60 0.035 205)',
      'oklch(0.67 0.03 205)',
      'oklch(0.74 0.025 205)',
      'oklch(0.81 0.02 205)',
      'oklch(0.68 0.01 245)'
    ]
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
