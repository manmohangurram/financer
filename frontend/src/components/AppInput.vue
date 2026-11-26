<script setup lang="ts">
import { useSlots } from 'vue';

defineProps<{
  label?: string;
  type?: string;
  placeholder?: string;
  hint?: string;
  error?: string;
  invalid?: boolean;
  step?: string;
  min?: string;
  minlength?: string;
  autocomplete?: string;
  required?: boolean;
}>();

const model = defineModel<string | number>();
const slots = useSlots();
</script>

<template>
  <label class="flex w-full flex-col gap-1.5">
    <span v-if="label" class="text-label font-medium uppercase tracking-[0.1em] text-ink-mute">{{ label }}</span>
    <span class="relative block">
      <slot name="icon" />
      <input
        v-model="model"
        :type="type || 'text'"
        :placeholder="placeholder"
        :step="step"
        :min="min"
        :minlength="minlength"
        :autocomplete="autocomplete"
        :required="required"
        :aria-invalid="!!error || invalid"
        class="input h-[38px] w-full rounded-control border bg-page-sunk px-3 text-[14px] text-ink placeholder:text-ink-faint transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20"
        :class="[slots.icon ? 'pl-9' : '', error || invalid ? 'border-debit' : 'border-rule']"
      />
      <slot name="dropdown" />
    </span>
    <span v-if="error" class="text-[12px] text-debit">{{ error }}</span>
    <span v-else-if="hint" class="text-[12px] text-ink-mute">{{ hint }}</span>
  </label>
</template>
