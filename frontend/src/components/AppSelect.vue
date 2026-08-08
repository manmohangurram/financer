<script setup lang="ts">
defineProps<{ label?: string; hint?: string }>();
const model = defineModel<string | number>();
</script>

<template>
  <label class="block w-full">
    <span v-if="label" class="block text-[12px] text-text-muted mb-1.5">{{ label }}</span>
    <select v-model="model" class="select w-full bg-surface border-border text-text text-[13px]">
      <slot />
    </select>
    <span v-if="hint" class="block text-[11px] text-subtle mt-1">{{ hint }}</span>
  </label>
</template>
