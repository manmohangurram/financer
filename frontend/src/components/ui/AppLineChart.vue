<script setup lang="ts">
import { computed } from 'vue';
import { formatCurrency } from '@/lib/utils/format';

export interface Bucket {
  key: string;
  label: string;
  amount: number;
}

// Spending over time: one line, hairline gridlines, a dot you can click.
const props = withDefaults(
  defineProps<{ buckets: Bucket[]; selectedKey?: string | null; height?: number }>(),
  { selectedKey: null, height: 116 }
);
const emit = defineEmits<{ (e: 'select', key: string): void }>();

const W = 640;
const PAD = { top: 16, right: 16, bottom: 16, left: 16 };

const max = computed(() => Math.max(1, ...props.buckets.map((b) => b.amount)));
const points = computed(() =>
  props.buckets.map((b, i) => {
    const span = Math.max(1, props.buckets.length - 1);
    const x = PAD.left + (i / span) * (W - PAD.left - PAD.right);
    const y = PAD.top + (1 - b.amount / max.value) * (props.height - PAD.top - PAD.bottom);
    return { ...b, x, y };
  })
);
const line = computed(() => points.value.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' '));
const grid = computed(() => [0, 0.5, 1].map((f) => PAD.top + f * (props.height - PAD.top - PAD.bottom)));

/** API labels are raw keys; the axis wants "8 Nov" / "Nov 2026". */
function axisLabel(label: string): string {
  if (/^\d{4}-\d{2}-\d{2}$/.test(label)) {
    const d = new Date(`${label}T00:00:00`);
    return d.toLocaleDateString(undefined, { day: 'numeric', month: 'short' });
  }
  if (/^\d{4}-\d{2}$/.test(label)) {
    const d = new Date(`${label}-01T00:00:00`);
    return d.toLocaleDateString(undefined, { month: 'short', year: 'numeric' });
  }
  return label;
}
</script>

<template>
  <div class="w-full">
    <svg :viewBox="`0 0 ${W} ${height}`" class="w-full" :style="{ height: `${height}px` }" role="img" aria-label="Spending over time">
      <line
        v-for="y in grid"
        :key="y"
        :x1="PAD.left"
        :x2="W - PAD.right"
        :y1="y"
        :y2="y"
        stroke="var(--rule-soft)"
        stroke-width="1"
        vector-effect="non-scaling-stroke"
      />
      <path :d="line" fill="none" stroke="var(--accent)" stroke-width="1.5" vector-effect="non-scaling-stroke" />
      <text :x="W - PAD.right" :y="PAD.top - 5" text-anchor="end" font-size="10" fill="var(--ink-faint)" font-family="var(--font-ui)">
        {{ formatCurrency(max) }}
      </text>
      <g v-for="p in points" :key="p.key">
        <rect
          :x="p.x - 8"
          :y="PAD.top"
          width="16"
          :height="height - PAD.top - PAD.bottom"
          fill="transparent"
          class="cursor-pointer"
          @click="emit('select', p.key)"
        >
          <title>{{ p.label }} · {{ formatCurrency(p.amount) }}</title>
        </rect>
        <circle
          :cx="p.x"
          :cy="p.y"
          :r="selectedKey === p.key ? 4 : 2.5"
          :fill="selectedKey === p.key ? 'var(--accent)' : 'var(--page)'"
          :stroke="selectedKey === p.key ? 'var(--accent)' : 'var(--accent)'"
          stroke-width="1.5"
          class="cursor-pointer"
          @click="emit('select', p.key)"
        />
      </g>
    </svg>
    <div class="flex justify-between text-[11px] text-ink-faint">
      <span v-if="buckets.length">{{ axisLabel(buckets[0].label) }}</span>
      <span v-if="buckets.length > 1">{{ axisLabel(buckets[buckets.length - 1].label) }}</span>
    </div>
  </div>
</template>
