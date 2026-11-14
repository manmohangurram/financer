<script setup lang="ts">
// Static bars in the rhythm of the text they stand in for. No shimmer: the
// page is paper, not a loading spinner.
withDefaults(defineProps<{ lines?: number; width?: string }>(), { lines: 3, width: 'w-2/3' });
</script>

<template>
  <div class="flex flex-col gap-2">
    <span
      v-for="i in lines"
      :key="i"
      class="block h-2.5 animate-pulse rounded-full bg-page-sunk"
      :class="i === lines ? 'w-1/3' : width"
    />
  </div>
</template>
