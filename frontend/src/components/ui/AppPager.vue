<script setup lang="ts">
import { computed } from 'vue';
import { ChevronLeft, ChevronRight } from '@lucide/vue';

// The one pager. Words, not carets: "Page 2 of 5" explains itself, and the two
// buttons sit at the card's edges where the thumb already is — the same bar at
// 360px and at 1440px. No row counts: what is being counted is money, and the
// band above already says it.
const props = defineProps<{
  /** 0-based. */
  page: number;
  totalPages: number;
  /** Highest page whose cursor is known; the transactions API is cursor based. */
  reachable?: number;
  /** What is being paged, for the landmark's name. */
  label?: string;
}>();

const emit = defineEmits<{ (e: 'page-change', page: number): void }>();

const lastReachable = computed(() => {
  const value = props.reachable ?? props.totalPages - 1;
  return Math.max(0, Math.min(value, props.totalPages - 1));
});

function go(p: number) {
  if (p < 0 || p > lastReachable.value || p === props.page) return;
  emit('page-change', p);
}

// 36px targets on a phone, 32px from sm up, so the footer never outgrows the
// carried-forward line above it.
const stepClass =
  'btn btn-ghost h-9 min-h-0 gap-1 rounded-control border px-2 text-[12px] font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-35 sm:h-8 sm:px-3';
const prevClass = `${stepClass} border-rule text-ink-soft hover:bg-page-sunk hover:text-ink disabled:hover:bg-transparent`;
const nextClass = `${stepClass} border-accent/40 bg-accent-wash text-accent hover:bg-accent/15 disabled:hover:bg-accent-wash`;
</script>

<template>
  <nav class="flex items-center justify-between gap-2" :aria-label="label ? `Pagination, ${label}` : 'Pagination'">
    <button type="button" :class="prevClass" :disabled="page <= 0" @click="go(page - 1)">
      <ChevronLeft class="h-[14px] w-[14px]" stroke-width="1.8" aria-hidden="true" />
      Prev
    </button>

    <p class="num text-[12px] text-ink-mute" aria-live="polite">
      Page <span class="font-semibold text-ink">{{ page + 1 }}</span> of
      <span class="font-semibold text-ink">{{ totalPages }}</span>
    </p>

    <button type="button" :class="nextClass" :disabled="page >= lastReachable" @click="go(page + 1)">
      Next
      <ChevronRight class="h-[14px] w-[14px]" stroke-width="1.8" aria-hidden="true" />
    </button>
  </nav>
</template>
