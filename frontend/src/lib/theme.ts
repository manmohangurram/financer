import { ref } from 'vue';

export type Theme = 'dark' | 'light';
export const STORAGE_KEY = 'financer-theme';

// The active theme. Reacts so UI (icons, category colors) updates on toggle.
// initTheme reads the data-theme already set by index.html's inline script
// (which applies the persisted choice before the app loads, avoiding a flash).
const current = ref<Theme>(initTheme());

function initTheme(): Theme {
  const t = document.documentElement.dataset.theme;
  return t === 'light' ? 'light' : 'dark';
}

export function applyTheme(t: Theme) {
  current.value = t;
  document.documentElement.dataset.theme = t;
  try {
    localStorage.setItem(STORAGE_KEY, t);
  } catch {
    /* storage unavailable — theme still applies for this session */
  }
}

export { current };
