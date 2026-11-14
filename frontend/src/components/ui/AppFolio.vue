<script setup lang="ts">
// A page of paper: hairline header, body, optional footer. No shadow — a card
// is a page, not a button.
defineProps<{ title?: string; subtitle?: string; flush?: boolean }>();
</script>

<template>
  <section class="rounded-card border border-rule bg-page">
    <header v-if="title || $slots.actions" class="flex items-center justify-between gap-4 border-b border-rule px-4 py-2.5">
      <div class="min-w-0">
        <h2 class="truncate text-[15px] font-semibold text-ink">{{ title }}</h2>
        <p v-if="subtitle" class="mt-0.5 truncate text-[12px] text-ink-mute">{{ subtitle }}</p>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <slot name="actions" />
      </div>
    </header>
    <div :class="flush ? '' : 'px-4 py-3.5'">
      <slot />
    </div>
    <footer v-if="$slots.footer" class="border-t border-rule px-4 py-2.5">
      <slot name="footer" />
    </footer>
  </section>
</template>
