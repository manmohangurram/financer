<script setup lang="ts">
// A labelled figure. The label is tracked caps, the value is mono/num, the
// note is the one line that says what the number is measured against.
withDefaults(
  defineProps<{
    label: string;
    value: string;
    note?: string;
    tone?: 'default' | 'credit' | 'debit' | 'muted';
    size?: 'sm' | 'md' | 'lg';
  }>(),
  { tone: 'default', size: 'md' }
);

const TONES: Record<string, string> = {
  default: 'text-ink',
  credit: 'text-credit',
  debit: 'text-debit',
  muted: 'text-ink-soft'
};
const SIZES: Record<string, string> = {
  sm: 'text-[14px]',
  md: 'text-[19px]',
  lg: 'text-[27px]'
};
</script>

<template>
  <div class="min-w-0">
    <p class="text-label font-medium uppercase tracking-[0.1em] text-ink-mute">{{ label }}</p>
    <p class="num mt-1.5 leading-none" :class="[SIZES[size], TONES[tone]]">{{ value }}</p>
    <p v-if="note" class="mt-1.5 truncate text-[11px] text-ink-mute">{{ note }}</p>
  </div>
</template>
