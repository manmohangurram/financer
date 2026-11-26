<script setup lang="ts">
import { computed, ref } from 'vue';
import { formatCurrency } from '@/lib/utils/format';

// One 100% bar. Flex-grow carries the proportion, so it always fills the box
// edge to edge; hovering a segment shows its name, amount and share.
const props = defineProps<{ items: { name: string; amount: number }[]; height?: number }>();

const height = computed(() => props.height ?? 30);
const total = computed(() => props.items.reduce((sum, i) => sum + i.amount, 0) || 1);

const hover = ref<{ name: string; amount: number; x: number; y: number } | null>(null);

function share(amount: number): number {
  return (amount / total.value) * 100;
}

function onEnter(item: { name: string; amount: number }, ev: MouseEvent) {
  hover.value = { name: item.name, amount: item.amount, x: ev.clientX, y: ev.clientY };
}
function onMove(ev: MouseEvent) {
  if (hover.value) hover.value = { ...hover.value, x: ev.clientX, y: ev.clientY };
}
function onLeave() {
  hover.value = null;
}
</script>

<template>
  <div class="relative">
    <div
      class="flex w-full gap-[3px]"
      role="img"
      :aria-label="items.map((i) => `${i.name} ${formatCurrency(i.amount)}`).join(', ')"
    >
      <button
        v-for="(item, i) in items"
        :key="item.name"
        type="button"
        class="rounded-[3px] transition-opacity hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
        :style="{
          flex: `${item.amount / total} 1 0`,
          height: `${height}px`,
          background: `var(--chart-${Math.min(i, 7)})`
        }"
        :aria-label="`${item.name}: ${formatCurrency(item.amount)} (${share(item.amount).toFixed(0)}%)`"
        @mouseenter="onEnter(item, $event)"
        @mousemove="onMove"
        @mouseleave="onLeave"
        @focus="hover = { name: item.name, amount: item.amount, x: 0, y: 0 }"
        @blur="onLeave"
      />
    </div>

    <div
      v-if="hover && hover.x"
      class="pointer-events-none fixed z-50 whitespace-nowrap rounded-control border border-rule bg-page px-2 py-1 text-[11.5px] font-medium text-ink shadow-lift"
      :style="{ left: `${hover.x + 12}px`, top: `${hover.y + 14}px` }"
      role="status"
    >
      {{ hover.name }} · {{ formatCurrency(hover.amount) }}
      <span class="text-ink-mute">{{ share(hover.amount).toFixed(0) }}%</span>
    </div>
  </div>
</template>
