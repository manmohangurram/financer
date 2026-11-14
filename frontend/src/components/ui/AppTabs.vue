<script setup lang="ts">
export interface TabItem {
  key: string;
  label: string;
  count?: number;
}

// Underline tabs only: a label and a rule, no filled pills.
defineProps<{ items: TabItem[] }>();
const model = defineModel<string>({ default: '' });
</script>

<template>
  <div class="flex items-end gap-5 border-b border-rule" role="tablist">
    <button
      v-for="item in items"
      :key="item.key"
      type="button"
      role="tab"
      :aria-selected="model === item.key"
      class="relative -mb-px pb-2 text-[13px] font-medium transition-colors"
      :class="model === item.key ? 'text-ink' : 'text-ink-mute hover:text-ink-soft'"
      @click="model = item.key"
    >
      {{ item.label }}
      <span v-if="item.count !== undefined" class="tabular ml-1 text-[11px] text-ink-faint">{{ item.count }}</span>
      <span
        v-if="model === item.key"
        class="absolute inset-x-0 bottom-0 h-[2px] rounded-full bg-accent"
        aria-hidden="true"
      ></span>
    </button>
  </div>
</template>
