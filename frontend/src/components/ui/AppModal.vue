<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { X } from '@lucide/vue';

// A sheet of paper over the page. Esc and the scrim close it; focus is trapped
// and handed back to whatever opened it.
const props = withDefaults(defineProps<{ open: boolean; title?: string; width?: 'form' | 'wide' }>(), {
  width: 'form'
});
const emit = defineEmits<{ (e: 'close'): void }>();

const panel = ref<HTMLElement | null>(null);
let opener: HTMLElement | null = null;

function focusables(): HTMLElement[] {
  if (!panel.value) return [];
  return Array.from(
    panel.value.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    )
  );
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault();
    emit('close');
    return;
  }
  if (e.key !== 'Tab') return;
  const items = focusables();
  if (!items.length) return;
  const first = items[0];
  const last = items[items.length - 1];
  if (e.shiftKey && document.activeElement === first) {
    e.preventDefault();
    last.focus();
  } else if (!e.shiftKey && document.activeElement === last) {
    e.preventDefault();
    first.focus();
  }
}

watch(
  () => props.open,
  async (isOpen) => {
    if (isOpen) {
      opener = document.activeElement as HTMLElement | null;
      document.addEventListener('keydown', onKeydown);
      await nextTick();
      // Prefer the first field: the reason the dialog was opened is usually to fill it.
      const firstField = panel.value?.querySelector<HTMLElement>('input, textarea, select');
      (firstField ?? focusables()[0] ?? panel.value)?.focus();
    } else {
      document.removeEventListener('keydown', onKeydown);
      opener?.focus?.();
    }
  }
);

onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown));
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-[70] flex items-center justify-center p-4">
      <div class="absolute inset-0 bg-overlay" @click="emit('close')" aria-hidden="true"></div>
      <div
        ref="panel"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
        tabindex="-1"
        class="relative flex max-h-[85vh] w-full flex-col rounded-overlay border border-rule bg-page shadow-lift"
        :class="width === 'wide' ? 'max-w-[760px]' : 'max-w-[520px]'"
      >
        <header class="flex items-center justify-between gap-4 border-b border-rule px-5 py-3.5">
          <h2 class="text-[15px] font-semibold text-ink">{{ title }}</h2>
          <button
            type="button"
            class="flex h-8 w-8 items-center justify-center rounded-control text-ink-mute transition-colors hover:bg-page-sunk hover:text-ink"
            aria-label="Close"
            @click="emit('close')"
          >
            <X class="h-4 w-4" stroke-width="1.8" />
          </button>
        </header>
        <div class="flex-1 overflow-auto px-5 py-4">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="flex items-center justify-end gap-2 border-t border-rule px-5 py-3">
          <slot name="footer" />
        </footer>
      </div>
    </div>
  </Teleport>
</template>
