import { reactive } from 'vue';

export interface Toast {
  id: number;
  message: string;
  tone: 'default' | 'error';
  action?: { label: string; run: () => void };
}

// One toast at a time is enough for this app: a second message replaces the
// first rather than stacking up in the corner.
export const toasts = reactive<Toast[]>([]);
let nextId = 1;
let timer: ReturnType<typeof setTimeout> | undefined;

export function toast(message: string, options: { tone?: 'default' | 'error'; action?: Toast['action'] } = {}) {
  toasts.splice(0, toasts.length, { id: nextId++, message, tone: options.tone ?? 'default', action: options.action });
  if (timer) clearTimeout(timer);
  timer = setTimeout(dismiss, 4000);
}

export function dismiss() {
  if (timer) clearTimeout(timer);
  timer = undefined;
  toasts.splice(0, toasts.length);
}
