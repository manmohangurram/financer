<script setup lang="ts">
// The only pill in the product. One word, hairline border, mono type.
withDefaults(
  defineProps<{
    tone?: 'neutral' | 'pending' | 'credit' | 'debit' | 'accent';
  }>(),
  { tone: 'neutral' }
);

const TONES: Record<string, string> = {
  neutral: 'border-rule-strong text-ink-mute',
  pending: 'border-stamp/50 text-stamp',
  credit: 'border-credit/40 text-credit',
  debit: 'border-debit/40 text-debit',
  accent: 'border-accent/40 text-accent'
};
</script>

<template>
  <span
    class="inline-flex items-center rounded-full border px-2 py-[3px] font-mono text-[10px] uppercase leading-none tracking-[0.04em]"
    :class="TONES[tone]"
  >
    <slot />
  </span>
</template>
