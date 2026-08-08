<script setup lang="ts">
defineProps<{ label: string; value: string; valueClass?: string; desc?: string }>();
</script>

<template>
  <div class="stats bg-base-200 border border-border shadow">
    <div class="stat">
      <div class="stat-title text-subtle">{{ label }}</div>
      <div class="stat-value text-[26px]" :class="valueClass || 'text-base-content'">{{ value }}</div>
      <div v-if="desc" class="stat-desc text-faint">{{ desc }}</div>
    </div>
  </div>
</template>
