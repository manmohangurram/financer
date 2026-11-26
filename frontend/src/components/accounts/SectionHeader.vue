<script setup lang="ts">
import AppPageHeader from '@/components/ui/AppPageHeader.vue';

// Kept for the existing pages that already use it; it is AppPageHeader, so the
// app has exactly one page-head implementation.
defineProps<{ title: string; subtitle?: string; eyebrow?: string }>();
</script>

<template>
  <AppPageHeader :title="title" :subtitle="subtitle" :eyebrow="eyebrow">
    <slot name="actions" />
  </AppPageHeader>
</template>
