<script setup lang="ts">
// The ruled page: a fact and the next action, nothing else.
defineProps<{ title: string; hint?: string }>();
</script>

<template>
  <div class="flex flex-col items-center gap-2 py-4">
    <p class="text-[13px] text-ink">{{ title }}</p>
    <p v-if="hint" class="max-w-[42ch] text-center text-[12px] text-ink-mute">{{ hint }}</p>
    <div v-if="$slots.action" class="mt-1">
      <slot name="action" />
    </div>
  </div>
</template>
