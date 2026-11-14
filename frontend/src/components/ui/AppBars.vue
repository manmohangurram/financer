<script setup lang="ts">
import { computed } from 'vue';
import { formatCurrency } from '@/lib/utils/format';

export interface Bar {
  key: string;
  label: string;
  amount: number;
}

// Category spending as ruled bars: one hairline box per row, filled from the
// left. Replaces the pie — comparison beats share for anything you act on.
const props = withDefaults(defineProps<{ bars: Bar[]; limit?: number; layout?: 'row' | 'stacked' }>(), {
  limit: 8,
  layout: 'row'
});

const max = computed(() => Math.max(1, ...props.bars.map((b) => b.amount)));
const shown = computed(() => props.bars.slice(0, props.limit));
const hidden = computed(() => Math.max(0, props.bars.length - props.limit));
</script>

<template>
  <div class="flex flex-col" :class="layout === 'stacked' ? 'gap-2.5' : 'gap-1.5'">
    <div v-for="bar in shown" :key="bar.key">
      <template v-if="layout === 'stacked'">
        <div class="flex items-baseline justify-between gap-2">
          <span class="truncate text-[12px] text-ink">{{ bar.label }}</span>
          <span class="tabular shrink-0 text-[12px] text-ink-soft">{{ formatCurrency(bar.amount) }}</span>
        </div>
        <span class="mt-1 block h-[6px] w-full overflow-hidden rounded-[1px] bg-page-sunk">
          <span
            class="block h-full border-l-2 border-accent bg-accent/25"
            :style="{ width: `${Math.max(2, (bar.amount / max) * 100)}%` }"
          />
        </span>
      </template>
      <div v-else class="grid grid-cols-[130px_minmax(0,1fr)_88px] items-center gap-3">
        <span class="truncate text-[13px] text-ink">{{ bar.label }}</span>
        <span class="relative block h-[12px] max-w-[420px] overflow-hidden rounded-[2px] border border-rule-soft">
          <span
            class="absolute inset-y-0 left-0 border-l-2 border-accent bg-accent/15"
            :style="{ width: `${Math.max(2, (bar.amount / max) * 100)}%` }"
          />
        </span>
        <span class="tabular text-right text-[13px] font-medium text-ink">{{ formatCurrency(bar.amount) }}</span>
      </div>
    </div>
    <p v-if="hidden" class="text-[11px] text-ink-mute">{{ hidden }} more categories</p>
  </div>
</template>
