<script setup lang="ts">
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from '@lucide/vue';

const props = defineProps<{ page: number; totalPages: number; label?: string }>();
const emit = defineEmits<{ (e: 'page-change', page: number): void }>();

function go(p: number) {
  if (p < 0 || p >= props.totalPages || p === props.page) return;
  emit('page-change', p);
}
</script>

<template>
  <div class="flex flex-col items-center gap-1.5 pt-3">
    <span v-if="label" class="text-[12px] text-subtle">{{ label }}</span>
    <div class="flex items-center gap-1.5" role="navigation" aria-label="Pagination">
      <button
        type="button"
        class="btn btn-ghost btn-xs border border-track p-1.5"
        :disabled="page <= 0"
        aria-label="First page"
        @click="go(0)"
      >
        <ChevronsLeft class="w-3.5 h-3.5" />
      </button>
      <button
        type="button"
        class="btn btn-ghost btn-xs border border-track p-1.5"
        :disabled="page <= 0"
        aria-label="Previous page"
        @click="go(page - 1)"
      >
        <ChevronLeft class="w-3.5 h-3.5" />
      </button>
      <span class="px-3 py-1 text-[12px] font-medium text-text-secondary">{{ page + 1 }} / {{ totalPages }}</span>
      <button
        type="button"
        class="btn btn-ghost btn-xs border border-track p-1.5"
        :disabled="page >= totalPages - 1"
        aria-label="Next page"
        @click="go(page + 1)"
      >
        <ChevronRight class="w-3.5 h-3.5" />
      </button>
      <button
        type="button"
        class="btn btn-ghost btn-xs border border-track p-1.5"
        :disabled="page >= totalPages - 1"
        aria-label="Last page"
        @click="go(totalPages - 1)"
      >
        <ChevronsRight class="w-3.5 h-3.5" />
      </button>
    </div>
  </div>
</template>
