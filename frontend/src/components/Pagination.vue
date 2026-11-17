<script setup lang="ts">
import { computed } from 'vue';
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from '@lucide/vue';

const props = defineProps<{
  page: number;
  totalPages: number;
  totalCount?: number;
  pageSize?: number;
  label?: string;
  /** Highest page whose cursor is known; the API is cursor based. */
  reachable?: number;
}>();

const emit = defineEmits<{ (e: 'page-change', page: number): void }>();

const lastReachable = computed(() => {
  const value = props.reachable ?? props.totalPages - 1;
  return Math.max(0, Math.min(value, props.totalPages - 1));
});

const from = computed(() => (props.totalCount ? props.page * (props.pageSize || 0) + 1 : 0));
const to = computed(() => {
  if (!props.totalCount || !props.pageSize) return 0;
  return Math.min(props.totalCount, (props.page + 1) * props.pageSize);
});

function go(p: number) {
  if (p < 0 || p > lastReachable.value || p === props.page) return;
  emit('page-change', p);
}

/** First, last reachable and the current page ±1, with ellipses for the gaps. */
const pages = computed<Array<number | '…'>>(() => {
  const reach = lastReachable.value;
  const current = props.page;
  const items: Array<number | '…'> = [0];
  const start = Math.max(1, current - 1);
  const end = Math.min(reach - 1, current + 1);
  if (start > 1) items.push('…');
  for (let i = start; i <= end; i++) items.push(i);
  if (end < reach - 1) items.push('…');
  if (reach > 0) items.push(reach);
  return items;
});

const buttonClass =
  'grid h-10 min-w-10 place-items-center rounded-control border border-rule px-2 text-[13px] tabular transition-colors hover:bg-page-sunk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 disabled:cursor-not-allowed disabled:opacity-35 disabled:hover:bg-transparent';
</script>

<template>
  <nav
    class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-t border-rule bg-page px-4 py-2.5"
    aria-label="Pagination"
  >
    <p class="text-[12.5px] text-ink-mute" aria-live="polite">
      <template v-if="totalCount && pageSize">
        <span class="tabular font-medium text-ink-soft">{{ from }}–{{ to }}</span>
        of <span class="tabular">{{ totalCount.toLocaleString() }}</span>
      </template>
      <template v-else-if="label">{{ label }}</template>
    </p>

    <div class="flex items-center gap-1">
      <button type="button" :class="buttonClass" :disabled="page <= 0" aria-label="First page" @click="go(0)">
        <ChevronsLeft class="h-4 w-4" stroke-width="1.8" />
      </button>
      <button type="button" :class="buttonClass" :disabled="page <= 0" aria-label="Previous page" @click="go(page - 1)">
        <ChevronLeft class="h-4 w-4" stroke-width="1.8" />
      </button>

      <span class="px-1 text-[12.5px] text-ink-soft sm:hidden">Page {{ page + 1 }} / {{ totalPages }}</span>

      <template v-for="(p, i) in pages" :key="`${p}-${i}`">
        <span v-if="p === '…'" class="hidden h-10 w-6 place-items-center text-[13px] text-ink-faint sm:grid" aria-hidden="true">…</span>
        <button
          v-else
          type="button"
          class="hidden h-10 min-w-10 place-items-center rounded-control border px-2 text-[13px] font-medium tabular transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 sm:grid"
          :class="p === page ? 'border-transparent bg-page-sunk text-ink' : 'border-rule text-ink-soft hover:bg-page-sunk'"
          :aria-current="p === page ? 'page' : undefined"
          :aria-label="`Page ${p + 1}`"
          @click="go(p)"
        >
          {{ p + 1 }}
        </button>
      </template>

      <button type="button" :class="buttonClass" :disabled="page >= lastReachable" aria-label="Next page" @click="go(page + 1)">
        <ChevronRight class="h-4 w-4" stroke-width="1.8" />
      </button>
      <button type="button" :class="buttonClass" :disabled="page >= lastReachable" aria-label="Last reachable page" @click="go(lastReachable)">
        <ChevronsRight class="h-4 w-4" stroke-width="1.8" />
      </button>
    </div>
  </nav>
</template>
