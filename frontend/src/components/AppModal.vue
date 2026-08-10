<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
defineProps<{ title: string; wide?: boolean; xwide?: boolean }>();
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
  <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60" @click.self="emit('close')">
    <div
      ref="panelEl"
      role="dialog"
      aria-modal="true"
      :aria-label="title"
      class="w-full rounded-2xl bg-gradient-to-br from-surface to-surface-alt border border-border"
      :class="xwide ? 'max-w-4xl' : wide ? 'max-w-2xl' : 'max-w-lg'"
    >
      <div class="flex items-center justify-between px-6 py-5 border-b border-border">
        <h3 class="text-lg font-semibold text-text">{{ title }}</h3>
        <button @click="emit('close')" aria-label="Close" class="text-subtle hover:text-text-secondary text-xl">&times;</button>
      </div>
      <slot />
    </div>
  </div>
</template>
