<script setup lang="ts">
import { computed } from 'vue';
import { Loader2 } from '@lucide/vue';

// The only button. Solid is the single primary action on a view; the accent
// carries it and the label colour comes from --on-accent so both themes work.
const props = withDefaults(
  defineProps<{
    variant?: 'solid' | 'outline' | 'quiet' | 'danger' | 'dangerSolid' | 'icon';
    size?: 'sm' | 'md';
    loading?: boolean;
    disabled?: boolean;
    type?: 'button' | 'submit';
    label?: string;
  }>(),
  {
    variant: 'solid',
    size: 'md',
    loading: false,
    disabled: false,
    type: 'button'
  }
);

const VARIANTS: Record<string, string> = {
  solid: 'bg-accent text-on-accent hover:bg-accent-hover border-transparent',
  outline: 'border-rule-strong bg-transparent text-ink hover:bg-page-sunk',
  quiet: 'btn-ghost border-transparent bg-transparent text-ink-soft hover:bg-page-sunk hover:text-ink',
  danger: 'border-debit/30 bg-transparent text-debit hover:bg-debit/10',
  dangerSolid: 'border-transparent bg-debit text-on-accent hover:bg-debit/90',
  icon: 'btn-ghost border-transparent bg-transparent text-ink-soft hover:bg-page-sunk hover:text-ink'
};

const classes = computed(() => [
  'btn inline-flex cursor-pointer items-center justify-center gap-2 rounded-control border font-medium shadow-none transition-colors',
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 focus-visible:ring-offset-1 focus-visible:ring-offset-page',
  'disabled:cursor-not-allowed disabled:opacity-50',
  props.variant === 'icon'
    ? 'h-11 w-11 p-0 md:h-9 md:w-9'
    : props.size === 'sm'
      ? 'h-8 px-3 text-[12px]'
      : 'h-[34px] px-3.5 text-[13px] max-md:min-h-11',
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
