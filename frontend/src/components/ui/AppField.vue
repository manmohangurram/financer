<script setup lang="ts">
import { computed } from 'vue';

// A ruled slot on the page: label above, hint below, reason on error.
const props = withDefaults(
  defineProps<{
    label?: string;
    hint?: string;
    error?: string;
    type?: string;
    placeholder?: string;
    autocomplete?: string;
    disabled?: boolean;
    readonly?: boolean;
    multiline?: boolean;
    rows?: number;
    id?: string;
  }>(),
  {
    type: 'text',
    disabled: false,
    readonly: false,
    multiline: false,
    rows: 3
  }
);

const model = defineModel<string | number>({ default: '' });

const fieldId = computed(() => props.id ?? `f-${Math.random().toString(36).slice(2, 9)}`);
const describedBy = computed(() => {
  const ids: string[] = [];
  if (props.hint) ids.push(`${fieldId.value}-hint`);
  if (props.error) ids.push(`${fieldId.value}-error`);
  return ids.length ? ids.join(' ') : undefined;
});

const classes = computed(() => [
  'w-full rounded-control border bg-page-sunk px-3 text-[14px] text-ink placeholder:text-ink-faint',
  'transition-colors focus:outline-none',
  props.error ? 'border-debit focus:border-debit' : 'border-rule focus:border-accent focus:ring-2 focus:ring-accent/20',
  props.readonly ? 'border-rule-soft bg-transparent' : '',
  'disabled:cursor-not-allowed disabled:opacity-60'
]);
</script>

<template>
  <div class="flex flex-col gap-1.5">
    <label v-if="label" :for="fieldId" class="text-label font-medium uppercase tracking-[0.1em] text-ink-mute">
      {{ label }}
    </label>

    <textarea
      v-if="multiline"
      :id="fieldId"
      v-model="model"
      :rows="rows"
      :placeholder="placeholder"
      :disabled="disabled"
      :readonly="readonly"
      :class="['textarea', ...classes, 'py-2 leading-normal']"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="describedBy"
    ></textarea>

    <input
      v-else
      :id="fieldId"
      v-model="model"
      :type="type"
      :placeholder="placeholder"
      :autocomplete="autocomplete"
      :disabled="disabled"
      :readonly="readonly"
      :class="['input', ...classes, 'h-[38px]']"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="describedBy"
    />

    <p v-if="error" :id="`${fieldId}-error`" class="text-[12px] text-debit">
      {{ error }}
    </p>
    <p v-else-if="hint" :id="`${fieldId}-hint`" class="text-[12px] text-ink-mute">
      {{ hint }}
    </p>
  </div>
</template>
