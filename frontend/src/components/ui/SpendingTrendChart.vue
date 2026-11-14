<script setup lang="ts">
import { computed } from 'vue';
import AppApexChart from '@/components/ui/AppApexChart.vue';
import { palette } from '@/lib/chartPalette';

// Spending over time: one smooth line, hairline grid, dots you can click.
const props = withDefaults(
  defineProps<{ points: { key: string; label: string; amount: number }[]; height?: number; selectedKey?: string | null; fill?: boolean }>(),
  { height: 220, selectedKey: null, fill: false }
);
const emit = defineEmits<{ (e: 'select', key: string): void }>();

let chartRef: { events?: Record<string, unknown> } | null = null;

const series = computed(() => [{ name: 'Spent', data: props.points.map((p) => Number(p.amount.toFixed(2))) }]);

const options = computed(() => ({
  chart: {
    type: 'line',
    events: {
      dataPointSelection: (_e: unknown, _ctx: unknown, cfg: { dataPointIndex: number }) => {
        const p = props.points[cfg.dataPointIndex];
        if (p) emit('select', p.key);
      }
    }
  },
  stroke: { curve: 'smooth', width: 1.6, colors: [palette.accent] },
  markers: {
    size: 3,
    strokeWidth: 1.4,
    strokeColors: palette.accent,
    colors: [palette.page],
    hover: { size: 5 }
  },
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
  grid: { borderColor: palette.rule, strokeDashArray: 0, xaxis: { lines: { show: false } } },
  tooltip: { y: { formatter: (v: number) => `₹${v.toLocaleString('en-IN')}` } },
  legend: { show: false },
  dataLabels: { enabled: false }
}));

void chartRef;
</script>

<template>
  <AppApexChart type="line" :series="series" :options="options" :height="height" :fill="fill" />
</template>
