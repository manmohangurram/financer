<script setup lang="ts">
import { computed } from 'vue';
import AppApexChart from '@/components/ui/AppApexChart.vue';
import { palette } from '@/lib/chartPalette';

// Composition as one 100% stacked bar (the daisyUI-charts "bar" pattern), with the
// amount legend rendered by the page — the bar carries share, the legend carries money.
const props = withDefaults(defineProps<{ items: { name: string; amount: number }[]; height?: number }>(), {
  height: 44
});

// Apex lays horizontal bars out right-to-left, so the array is reversed to put the
// biggest share at the left edge — the order the legend reads in.
const ordered = computed(() => [...props.items].reverse());
/** The axis must span exactly the total, or the segments don't tile the full width. */
const total = computed(() => props.items.reduce((sum, i) => sum + i.amount, 0) || 1);
const series = computed(() => [{ name: 'Spent', data: ordered.value.map((i) => Number(i.amount.toFixed(2))) }]);
const colors = computed(() =>
  ordered.value.map((_i, idx) => palette.ramp[Math.min(ordered.value.length - 1 - idx, palette.ramp.length - 1)])
);

const options = computed(() => ({
  chart: { type: 'bar', height: props.height, stacked: true },
  plotOptions: {
    bar: {
      horizontal: true,
      barHeight: 22,
      distributed: true,
      borderRadius: 3
    }
  },
  colors: colors.value,
  stroke: { width: 2, colors: [palette.page] },
  dataLabels: { enabled: false },
  legend: { show: false },
  grid: { show: false, padding: { left: 0, right: 0, top: 0, bottom: 0 } },
  xaxis: {
    min: 0,
    max: total.value,
    labels: { show: false },
    axisBorder: { show: false },
    axisTicks: { show: false },
    crosshairs: { show: false }
  },
  yaxis: { labels: { show: false } },
  tooltip: { enabled: false }
}));
</script>

<template>
  <div
    v-if="items.length < 2"
    class="w-full rounded-[3px] bg-accent"
    :style="{ height: `${22}px` }"
    :title="items[0] ? `${items[0].name}` : ''"
  />
  <AppApexChart v-else type="bar" :series="series" :options="options" :height="height" />
</template>
