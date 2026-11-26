<script setup lang="ts">
// The only pill in the product: one word, mono caps, tinted by meaning.
withDefaults(
  defineProps<{
    tone?: 'neutral' | 'pending' | 'credit' | 'debit' | 'accent';
  }>(),
  { tone: 'neutral' }
);

const TONES: Record<string, string> = {
  neutral: 'border-rule-strong text-ink-mute bg-page-sunk/60',
  pending: 'border-stamp/35 text-stamp bg-stamp/10',
  credit: 'border-credit/35 text-credit bg-credit/10',
  debit: 'border-debit/35 text-debit bg-debit/10',
  accent: 'border-accent/35 text-accent bg-accent/10'
};
</script>

<template>
  <span
    class="badge inline-flex items-center rounded-full border px-2 py-[3px] font-mono text-[10px] uppercase leading-none tracking-[0.04em]"
    :class="TONES[tone]"
  >
    <slot />
  </span>
</template>
