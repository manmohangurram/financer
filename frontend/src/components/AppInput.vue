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
  <label class="block w-full">
    <span v-if="label" class="block text-[12px] text-text-muted mb-1.5">{{ label }}</span>
    <div class="relative">
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
        class="input w-full bg-surface border-border text-text text-[13px]"
        :class="[slots.icon ? 'pl-9' : '', error || invalid ? 'border-expense/60' : '']"
      />
      <slot name="dropdown" />
    </div>
    <span v-if="hint" class="block text-[11px] text-subtle mt-1">{{ hint }}</span>
    <span v-if="error" class="block text-[11px] text-expense mt-1">{{ error }}</span>
  </label>
</template>
