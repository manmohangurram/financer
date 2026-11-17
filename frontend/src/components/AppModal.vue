<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
defineProps<{ title: string; wide?: boolean; xwide?: boolean; steady?: boolean }>();
const emit = defineEmits<{ (e: 'close'): void }>();

const panelEl = ref<HTMLElement | null>(null);
let previousFocus: HTMLElement | null = null;

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close');
}

onMounted(() => {
  previousFocus = document.activeElement as HTMLElement | null;
  const focusable = panelEl.value?.querySelector<HTMLElement>('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
  focusable?.focus();
  window.addEventListener('keydown', onKeydown);
});

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown);
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
      :class="[xwide ? 'max-w-4xl' : wide ? 'max-w-2xl' : 'max-w-lg', steady ? 'h-[min(85vh,660px)]' : '']"
    >
      <div class="flex shrink-0 items-center justify-between gap-4 border-b border-rule px-5 py-3.5">
        <h3 class="text-[15px] font-semibold text-ink">{{ title }}</h3>
        <button type="button" @click="emit('close')" aria-label="Close" class="flex h-8 w-8 items-center justify-center rounded-control text-ink-mute transition-colors hover:bg-page-sunk hover:text-ink">&times;</button>
      </div>
      <slot />
    </div>
  </div>
</template>
