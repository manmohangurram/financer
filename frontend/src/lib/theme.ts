import { ref, computed } from 'vue';

export type Theme = 'system' | 'dark' | 'light';
export type ResolvedTheme = 'dark' | 'light';
export const STORAGE_KEY = 'financer-theme';

// Reacts to OS-level preference changes while in 'system' mode.
const systemDark = window.matchMedia('(prefers-color-scheme: dark)');

// The user's chosen preference. initTheme reads localStorage because
// index.html's inline script already applied the resolved theme before the
// bundle loads (avoiding a flash), so data-theme holds dark/light, not the mode.
const current = ref<Theme>(initTheme());

function initTheme(): Theme {
  const t = localStorage.getItem(STORAGE_KEY);
  return t === 'light' || t === 'dark' ? t : 'system';
}

export function resolveTheme(t: Theme): ResolvedTheme {
  return t === 'system' ? (systemDark.matches ? 'dark' : 'light') : t;
}

// The theme actually applied to the document — used by UI that must react to
// what's on screen (category colors), regardless of the chosen mode.
export const resolved = computed<ResolvedTheme>(() => resolveTheme(current.value));

export function applyTheme(t: Theme) {
  current.value = t;
  document.documentElement.dataset.theme = resolveTheme(t);
  try {
    localStorage.setItem(STORAGE_KEY, t);
  } catch {
    /* storage unavailable — theme still applies for this session */
  }
}

systemDark.addEventListener('change', () => {
  if (current.value === 'system') document.documentElement.dataset.theme = resolveTheme('system');
});

export { current };
