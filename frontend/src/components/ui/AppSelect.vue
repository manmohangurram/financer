<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { Check, ChevronDown } from '@lucide/vue';

export interface SelectOption {
  value: string;
  label: string;
  hint?: string;
}

// A ruled slot that opens a paper panel — never a native <select>. The trigger
// is the same 38px field as AppField, so a form row of mixed controls lines up.
const props = withDefaults(
  defineProps<{
    label?: string;
    placeholder?: string;
    options: SelectOption[];
    hint?: string;
    error?: string;
    disabled?: boolean;
  }>(),
  { placeholder: 'Select…', disabled: false }
);

const model = defineModel<string>({ default: '' });
const open = ref(false);
const active = ref(-1);
const root = ref<HTMLElement | null>(null);

const selected = computed(() => props.options.find((o) => o.value === model.value) ?? null);
const listId = `sel-${Math.random().toString(36).slice(2, 9)}`;
const optionId = (i: number) => `${listId}-${i}`;

function toggle() {
  if (props.disabled) return;
  open.value = !open.value;
  if (open.value) {
    active.value = Math.max(
      0,
      props.options.findIndex((o) => o.value === model.value)
    );
  }
}

function choose(option: SelectOption) {
  model.value = option.value;
  open.value = false;
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    open.value = false;
    return;
  }
  if (e.key === 'Enter' || e.key === ' ') {
    e.preventDefault();
    if (!open.value) return toggle();
    const option = props.options[active.value];
    if (option) choose(option);
    return;
  }
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault();
    if (!open.value) return toggle();
    const step = e.key === 'ArrowDown' ? 1 : -1;
    active.value = (active.value + step + props.options.length) % props.options.length;
  }
}

function onDocClick(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) open.value = false;
}

watch(open, (isOpen) => {
  if (isOpen) document.addEventListener('mousedown', onDocClick);
  else document.removeEventListener('mousedown', onDocClick);
});
onBeforeUnmount(() => document.removeEventListener('mousedown', onDocClick));
</script>

<template>
  <div ref="root" class="relative flex flex-col gap-1.5">
    <span v-if="label" class="text-label font-medium uppercase tracking-[0.1em] text-ink-mute">{{ label }}</span>

    <button
      type="button"
      class="flex h-[38px] w-full items-center justify-between gap-2 rounded-control border bg-page-sunk px-3 text-left text-[14px] transition-colors focus:outline-none"
      :class="
        error
          ? 'border-debit'
          : open
            ? 'border-accent ring-2 ring-accent/20'
            : 'border-rule focus:border-accent focus:ring-2 focus:ring-accent/20'
      "
      :disabled="disabled"
      aria-haspopup="listbox"
      :aria-expanded="open"
      :aria-controls="open ? listId : undefined"
      :aria-activedescendant="open && active >= 0 ? optionId(active) : undefined"
      @click="toggle"
      @keydown="onKeydown"
    >
      <span :class="selected ? 'text-ink' : 'text-ink-faint'" class="truncate">{{
        selected?.label ?? placeholder
      }}</span>
      <ChevronDown
        class="h-[15px] w-[15px] shrink-0 text-ink-mute transition-transform"
        :class="open ? 'rotate-180' : ''"
        stroke-width="1.6"
        aria-hidden="true"
      />
    </button>

    <ul
      v-if="open"
      :id="listId"
      role="listbox"
      class="absolute left-0 right-0 top-full z-40 mt-1 max-h-64 overflow-auto rounded-control border border-rule bg-page p-1 shadow-lift"
    >
      <li
        v-for="(option, i) in options"
        :id="optionId(i)"
        :key="option.value"
        role="option"
        :aria-selected="option.value === model"
        class="flex h-8 cursor-pointer items-center gap-2 rounded-control px-2.5 text-[13px] transition-colors"
        :class="i === active ? 'bg-page-sunk text-ink' : 'text-ink-soft'"
        @mouseenter="active = i"
        @click="choose(option)"
      >
        <span class="flex-1 truncate">
          {{ option.label }}
          <span v-if="option.hint" class="ml-1 text-[11px] text-ink-faint">{{ option.hint }}</span>
        </span>
        <Check
          v-if="option.value === model"
          class="h-[14px] w-[14px] text-accent"
          stroke-width="2"
          aria-hidden="true"
        />
      </li>
      <li v-if="!options.length" class="px-2.5 py-2 text-[12px] text-ink-mute">Nothing to choose from</li>
    </ul>

    <p v-if="error" class="text-[12px] text-debit">{{ error }}</p>
    <p v-else-if="hint" class="text-[12px] text-ink-mute">{{ hint }}</p>
  </div>
</template>
