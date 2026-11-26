<script setup lang="ts">
import { computed } from 'vue';

// The signature interaction: a tick drawn in the ledger margin.
const props = withDefaults(
  defineProps<{
    state?: 'idle' | 'done' | 'refused';
    label: string;
  }>(),
  { state: 'idle' }
);

const emit = defineEmits<{ (e: 'click', ev: MouseEvent): void }>();

const tone = computed(
  () => ({ idle: 'text-ink-faint hover:text-accent', done: 'text-accent', refused: 'text-debit' })[props.state]
);
</script>

<template>
  <button
    type="button"
    class="relative flex h-11 w-11 items-center justify-center rounded-control transition-colors hover:bg-page-sunk"
    :class="tone"
    :aria-label="label"
    :aria-pressed="state === 'done'"
    :title="label"
    @click="emit('click', $event)"
  >
    <svg viewBox="0 0 20 20" class="h-[18px] w-[18px]" aria-hidden="true">
      <circle
        v-if="state === 'idle'"
        cx="10"
        cy="10"
        r="6.5"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-dasharray="2 2"
      />
      <polyline
        v-else-if="state === 'done'"
        points="4,10.5 8,14.5 16,5.5"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path v-else d="M5 5l10 10M15 5L5 15" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
    </svg>
  </button>
</template>
