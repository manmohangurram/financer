<script setup lang="ts">
defineProps<{ label?: string; hint?: string }>();
const model = defineModel<string | number>();
</script>

<template>
  <label class="flex w-full flex-col gap-1.5">
    <span v-if="label" class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">{{ label }}</span>
    <select
      v-model="model"
      class="h-[38px] w-full rounded-control border border-rule bg-page-sunk px-2.5 text-[14px] text-ink transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20"
    >
      <slot />
    </select>
    <span v-if="hint" class="text-[12px] text-ink-mute">{{ hint }}</span>
  </label>
</template>
