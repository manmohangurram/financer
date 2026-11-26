<script setup lang="ts">
import { onBeforeUnmount } from 'vue';
import { X } from '@lucide/vue';
import { toasts, dismiss } from '@/lib/toasts';

onBeforeUnmount(dismiss);
</script>

<template>
  <div
    class="pointer-events-none fixed bottom-4 right-4 z-[80] flex flex-col gap-2"
    aria-live="polite"
    aria-atomic="true"
  >
    <div
      v-for="t in toasts"
      :key="t.id"
      class="pointer-events-auto flex max-w-[360px] items-start gap-3 rounded-card border bg-page px-3.5 py-2.5 shadow-lift"
      :class="t.tone === 'error' ? 'border-debit/40' : 'border-rule'"
    >
      <p class="flex-1 text-[13px]" :class="t.tone === 'error' ? 'text-debit' : 'text-ink'">{{ t.message }}</p>
      <button
        v-if="t.action"
        type="button"
        class="text-[12px] font-medium text-accent hover:text-accent-hover"
        @click="
          t.action.run();
          dismiss();
        "
      >
        {{ t.action.label }}
      </button>
      <button
        type="button"
        class="text-ink-faint transition-colors hover:text-ink"
        aria-label="Dismiss"
        @click="dismiss()"
      >
        <X class="h-[14px] w-[14px]" stroke-width="1.8" />
      </button>
    </div>
  </div>
</template>
