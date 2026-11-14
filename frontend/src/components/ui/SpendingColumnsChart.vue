<script setup lang="ts">
import { computed } from 'vue';
import AppApexChart from '@/components/ui/AppApexChart.vue';
import { palette } from '@/lib/chartPalette';

// Spending over time as day/month columns: a bucket either has spending or it
// doesn't, and a column says so; a line would imply movement between days.
const props = withDefaults(
  defineProps<{ points: { key: string; label: string; amount: number }[]; height?: number; selectedKey?: string | null }>(),
  { height: 220, selectedKey: null }
);
const emit = defineEmits<{ (e: 'select', key: string): void }>();

const series = computed(() => [{ name: 'Spent', data: props.points.map((p) => Number(p.amount.toFixed(2))) }]);
const selectedIndex = computed(() => props.points.findIndex((p) => p.key === props.selectedKey));

const options = computed(() => ({
  chart: {
    type: 'bar',
    height: props.height,
    events: {
      dataPointSelection: (_e: unknown, _ctx: unknown, cfg: { dataPointIndex: number }) => {
        const p = props.points[cfg.dataPointIndex];
        if (p) emit('select', p.key);
      }
    }
  },
  plotOptions: {
    bar: {
      columnWidth: '58%',
      borderRadius: 2,
      borderRadiusApplication: 'end',
      distributed: true
    }
  },
  // everything solid until a day is picked; then the pick stays solid and the rest recede
  colors: computed(() =>
    props.points.map((_p, i) =>
      selectedIndex.value === -1 || i === selectedIndex.value ? palette.accent : palette.accentQuiet
    )
  ).value,
  dataLabels: { enabled: false },
  legend: { show: false },
  stroke: { width: 1, colors: [palette.rule] },
  grid: { borderColor: palette.rule, strokeDashArray: 0, xaxis: { lines: { show: false } } },
  xaxis: {
    categories: props.points.map((p) => p.label),
    axisBorder: { show: false },
    axisTicks: { show: false },
    labels: { style: { fontSize: '10px', colors: palette.inkMute }, hideOverlappingLabels: true },
    tooltip: { enabled: false }
  },
  yaxis: {
    labels: {
      style: { fontSize: '10px', colors: palette.inkMute },
      formatter: (v: number) => (v >= 1000 ? `₹${Math.round(v / 1000)}k` : `₹${Math.round(v)}`)
    }
  },
  tooltip: { y: { formatter: (v: number) => `₹${v.toLocaleString('en-IN')}` } }
}));
</script>

<template>
  <AppApexChart type="bar" :series="series" :options="options" :height="height" />
</template>
