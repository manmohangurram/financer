<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';

withDefaults(
  defineProps<{
    panelClass?: string;
    up?: boolean;
    right?: boolean;
    noClip?: boolean;
    label?: string;
  }>(),
  {
    panelClass: 'right-0 min-w-40',
    up: false,
    right: false,
    noClip: false,
    label: 'Menu'
  }
);
const emit = defineEmits<{ (e: 'close'): void }>();

const panelEl = ref<HTMLElement | null>(null);
let previousFocus: HTMLElement | null = null;

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.stopPropagation();
    emit('close');
  }
}

onMounted(() => {
  previousFocus = document.activeElement as HTMLElement | null;
  window.addEventListener('keydown', onKeydown);
  const first = panelEl.value?.querySelector<HTMLElement>(
    'input, select, textarea, button:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])'
  );
  (first ?? panelEl.value)?.focus();
});

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown);
  previousFocus?.focus();
});
</script>

<template>
  <div class="absolute inset-0 z-50">
    <div class="fixed inset-0 z-40" aria-hidden="true" @click="emit('close')"></div>
    <div
      class="absolute z-50"
      :class="[right ? 'bottom-0 left-full ml-2' : up ? 'bottom-full mb-2' : 'top-full mt-2', panelClass]"
    >
      <div
        ref="panelEl"
        role="dialog"
        :aria-label="label"
        tabindex="-1"
        class="rounded-control border border-rule bg-page shadow-lift focus:outline-none"
        :class="noClip ? '' : 'overflow-hidden'"
        @click.stop
      >
        <slot />
      </div>
    </div>
  </div>
</template>
