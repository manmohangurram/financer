<script setup lang="ts">
import { computed } from 'vue';
import { Loader2 } from '@lucide/vue';

const props = withDefaults(
  defineProps<{
    variant?: 'solid' | 'outline' | 'quiet' | 'danger' | 'icon';
    size?: 'sm' | 'md';
    loading?: boolean;
    disabled?: boolean;
    type?: 'button' | 'submit';
    label?: string;
  }>(),
  { variant: 'solid', size: 'md', loading: false, disabled: false, type: 'button' }
);

const VARIANTS: Record<string, string> = {
  solid: 'bg-accent text-paper hover:bg-accent-hover border-transparent',
  outline: 'border-rule-strong text-ink hover:bg-page-sunk',
  quiet: 'border-transparent text-ink-soft hover:bg-page-sunk hover:text-ink',
  danger: 'border-debit/30 text-debit hover:bg-debit/10',
  icon: 'border-transparent text-ink-soft hover:bg-page-sunk hover:text-ink'
};

const classes = computed(() => [
  'inline-flex items-center justify-center gap-2 rounded-control border font-medium transition-colors',
  'disabled:cursor-not-allowed disabled:opacity-50',
  props.variant === 'icon'
    ? 'h-8 w-8'
    : props.size === 'sm'
      ? 'h-8 px-3 text-[12px]'
      : 'h-[34px] px-3.5 text-[13px]',
  VARIANTS[props.variant]
]);
</script>

<template>
  <button
    :type="type"
    :class="classes"
    :disabled="disabled || loading"
    :aria-busy="loading || undefined"
    :aria-label="variant === 'icon' ? label : undefined"
    :title="variant === 'icon' ? label : undefined"
  >
    <Loader2 v-if="loading" class="h-[15px] w-[15px] animate-spin" stroke-width="2" aria-hidden="true" />
    <slot />
  </button>
</template>
