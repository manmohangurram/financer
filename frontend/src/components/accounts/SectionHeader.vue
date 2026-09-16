<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>();
</script>

<template>
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <h1 class="text-[24px] font-bold text-base-content tracking-tight">{{ title }}</h1>
      <p v-if="subtitle" class="text-[13.5px] text-subtle mt-0.5 truncate">{{ subtitle }}</p>
    </div>
    <div class="flex gap-2 shrink-0">
      <slot name="actions"></slot>
    </div>
  </div>
</template>
