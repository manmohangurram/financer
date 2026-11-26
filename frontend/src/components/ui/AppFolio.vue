<script setup lang="ts">
// A card: one elevated surface, one hairline head, one body. The optional
// footer is for a total or the single action that belongs to the card.
withDefaults(
  defineProps<{
    title?: string;
    subtitle?: string;
    flush?: boolean;
    bodyClass?: string;
  }>(),
  {
    bodyClass: ''
  }
);
</script>

<template>
  <section class="card flex flex-col overflow-hidden rounded-card border border-rule bg-page">
    <header
      v-if="title || $slots.actions"
      class="flex items-start justify-between gap-4 border-b border-rule px-4 pb-3 pt-3.5 md:px-5"
    >
      <div class="min-w-0">
        <h2 class="truncate text-[14px] font-semibold text-ink">
          {{ title }}
        </h2>
        <p v-if="subtitle" class="mt-1 truncate text-[11.5px] text-ink-mute">
          {{ subtitle }}
        </p>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <slot name="actions" />
      </div>
    </header>
    <div class="card-body min-h-0 flex-1" :class="[flush ? 'p-0' : 'px-4 py-4 md:px-5', bodyClass]">
      <slot />
    </div>
    <footer v-if="$slots.footer" class="shrink-0 border-t border-rule px-4 py-2.5 md:px-5">
      <slot name="footer" />
    </footer>
  </section>
</template>
