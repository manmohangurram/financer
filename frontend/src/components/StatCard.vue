<script setup lang="ts">
// One figure from the books: label above, value in the UI face, optional note.
defineProps<{ label: string; value: string; valueClass?: string; desc?: string }>();
</script>

<template>
  <div class="flex flex-col gap-1 rounded-card border border-rule bg-page px-4 py-3">
    <span class="text-label font-medium uppercase tracking-[0.07em] text-ink-mute">{{ label }}</span>
    <span class="tabular text-[19px] leading-none font-medium" :class="valueClass || 'text-ink'">{{ value }}</span>
    <span v-if="desc" class="text-[11.5px] text-ink-mute">{{ desc }}</span>
  </div>
</template>
