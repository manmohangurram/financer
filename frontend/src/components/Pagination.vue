<script setup lang="ts">
import { ChevronLeft, ChevronRight } from '@lucide/vue';

// `‹ 1 2 3 ›` with the current page filled — the same control everywhere.
const props = defineProps<{ page: number; totalPages: number; label?: string }>();
const emit = defineEmits<{ (e: 'page-change', page: number): void }>();

function go(p: number) {
  if (p < 0 || p >= props.totalPages || p === props.page) return;
  emit('page-change', p);
}

// At most five numbers, centred on the current page.
function pages(): number[] {
  const total = props.totalPages;
  const from = Math.max(0, Math.min(props.page - 2, total - 5));
  return Array.from({ length: Math.min(5, total) }, (_v, i) => from + i);
}
</script>

<template>
  <div class="flex items-center justify-between gap-3">
    <span v-if="label" class="text-[12px] text-ink-mute">{{ label }}</span>
    <div class="ml-auto flex items-center gap-1" role="navigation" aria-label="Pagination">
      <button
        type="button"
        class="flex h-[26px] min-w-[26px] items-center justify-center rounded-control border border-rule text-ink-soft transition-colors hover:bg-page-sunk disabled:opacity-40 disabled:hover:bg-transparent"
        :disabled="page <= 0"
        aria-label="Previous page"
        @click="go(page - 1)"
      >
        <ChevronLeft class="h-3.5 w-3.5" stroke-width="1.8" />
      </button>
      <button
        v-for="p in pages()"
        :key="p"
        type="button"
        class="tabular h-[26px] min-w-[26px] rounded-control border px-1 text-[12px] font-medium transition-colors"
        :class="p === page ? 'border-transparent bg-page-sunk text-ink' : 'border-rule text-ink-soft hover:bg-page-sunk'"
        :aria-current="p === page ? 'page' : undefined"
        @click="go(p)"
      >
        {{ p + 1 }}
      </button>
      <button
        type="button"
        class="flex h-[26px] min-w-[26px] items-center justify-center rounded-control border border-rule text-ink-soft transition-colors hover:bg-page-sunk disabled:opacity-40 disabled:hover:bg-transparent"
        :disabled="page >= totalPages - 1"
        aria-label="Next page"
        @click="go(page + 1)"
      >
        <ChevronRight class="h-3.5 w-3.5" stroke-width="1.8" />
      </button>
    </div>
  </div>
</template>
