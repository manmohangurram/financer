<script setup lang="ts">
export interface TimeRangeOption {
  id: string;
  label: string;
}

// Reusable time-range selector. Defaults to the shared ranges (7D/1M/6M/1Y +
// Custom) used across the app; consumers can pass their own options.
withDefaults(
  defineProps<{ options?: TimeRangeOption[] }>(),
  {
    options: () => [
      { id: '7D', label: '7D' },
      { id: '1M', label: '1M' },
      { id: '6M', label: '6M' },
      { id: '1Y', label: '1Y' },
      { id: 'CUSTOM', label: 'Custom' }
    ]
  }
);

const model = defineModel<string>();
</script>

<template>
  <div class="flex gap-1 rounded-xl bg-surface border border-border p-1" role="group" aria-label="Time range">
    <button
      v-for="opt in options"
      :key="opt.id"
      type="button"
      class="px-3 py-1.5 rounded-lg text-[12px] font-medium transition-colors"
      :class="model === opt.id ? 'bg-primary-600 text-white' : 'text-subtle hover:text-text'"
      @click="model = opt.id"
    >{{ opt.label }}</button>
  </div>
</template>
