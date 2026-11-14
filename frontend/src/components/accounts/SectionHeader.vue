<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>();
</script>

<template>
  <div class="flex items-end justify-between gap-4">
    <div class="min-w-0">
      <h1 class="text-[22px] leading-none text-ink">{{ title }}</h1>
      <p v-if="subtitle" class="mt-1 truncate text-[12.5px] text-ink-mute">{{ subtitle }}</p>
    </div>
    <div class="flex shrink-0 items-center gap-2">
      <slot name="actions"></slot>
    </div>
  </div>
</template>
