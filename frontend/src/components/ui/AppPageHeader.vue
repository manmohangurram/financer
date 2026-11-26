<script setup lang="ts">
// The one page header in the app: an optional eyebrow, the page title, and the
// page's actions on the right. Pages pass slots, never their own h1 styling.
withDefaults(
  defineProps<{
    title: string;
    eyebrow?: string;
    subtitle?: string;
  }>(),
  { eyebrow: '', subtitle: '' }
);
</script>

<template>
  <header class="flex flex-wrap items-end justify-between gap-3">
    <div class="min-w-0">
      <p v-if="eyebrow" class="mb-1.5 text-label font-medium uppercase tracking-[0.1em] text-ink-mute">
        {{ eyebrow }}
      </p>
      <h1 class="text-[22px] leading-tight text-ink md:text-[25px]">{{ title }}</h1>
      <p v-if="subtitle" class="mt-1 text-[12.5px] text-ink-mute">{{ subtitle }}</p>
    </div>
    <div class="flex flex-wrap items-center gap-2.5 pb-0.5">
      <slot />
    </div>
  </header>
</template>
