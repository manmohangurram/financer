<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { X } from '@lucide/vue';

defineProps<{
  title: string;
  wide?: boolean;
  xwide?: boolean;
  steady?: boolean;
  roomy?: boolean;
}>();
const emit = defineEmits<{ (e: 'close'): void }>();

const panelEl = ref<HTMLElement | null>(null);
let previousFocus: HTMLElement | null = null;
let previousOverflow = '';

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

function focusables(): HTMLElement[] {
  if (!panelEl.value) return [];
  return Array.from(panelEl.value.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (el) => el.offsetParent !== null || el === document.activeElement
  );
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    emit('close');
    return;
  }
  // Keep Tab inside the dialog; a modal must not hand focus back to the page.
  if (e.key === 'Tab') {
    const nodes = focusables();
    if (nodes.length === 0) {
      e.preventDefault();
      return;
    }
    const first = nodes[0];
    const last = nodes[nodes.length - 1];
    const active = document.activeElement as HTMLElement | null;
    if (e.shiftKey && (active === first || !panelEl.value?.contains(active))) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && active === last) {
      e.preventDefault();
      first.focus();
    }
  }
}

onMounted(() => {
  previousFocus = document.activeElement as HTMLElement | null;
  previousOverflow = document.body.style.overflow;
  document.body.style.overflow = 'hidden';
  const focusable = panelEl.value?.querySelector<HTMLElement>(FOCUSABLE);
  focusable?.focus();
  window.addEventListener('keydown', onKeydown);
});

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown);
  document.body.style.overflow = previousOverflow;
  previousFocus?.focus();
});
</script>

<template>
  <div class="fixed inset-0 z-[100] flex items-center justify-center bg-overlay p-4" @click.self="emit('close')">
    <div
      ref="panelEl"
      role="dialog"
      aria-modal="true"
      :aria-label="title"
      class="flex max-h-[85vh] w-full flex-col overflow-hidden rounded-overlay border border-rule bg-page shadow-lift"
      :class="[
        roomy ? 'max-w-[880px]' : xwide ? 'max-w-4xl' : wide ? 'max-w-2xl' : 'max-w-lg',
        steady ? 'h-[min(85vh,660px)]' : ''
      ]"
    >
      <div
        class="flex shrink-0 items-center justify-between gap-4 border-b border-rule"
        :class="roomy ? 'px-[26px] py-[18px]' : 'px-5 py-3.5'"
      >
        <h2 class="text-[15px] font-semibold text-ink">{{ title }}</h2>
        <button
          type="button"
          aria-label="Close"
          class="flex h-11 w-11 shrink-0 items-center justify-center rounded-control text-ink-mute transition-colors hover:bg-page-sunk hover:text-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 sm:h-8 sm:w-8"
          @click="emit('close')"
        >
          <X class="h-4 w-4" stroke-width="1.8" aria-hidden="true" />
        </button>
      </div>
      <slot />
    </div>
  </div>
</template>
