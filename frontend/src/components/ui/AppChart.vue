<script setup lang="ts">
import { computed } from 'vue';
import { palette } from '@/lib/chartPalette';
import { resolved as themeResolved } from '@/lib/theme';
import AppApexChart from './AppApexChart.vue';

// The one chart surface in the app. Pages pass data and a kind; the grid, axis
// type, accent, gradient and marker style are decided here, once, so every
// chart in a page reads as the same instrument.
export type ChartKind = 'area' | 'line';

const props = withDefaults(
  defineProps<{
    kind?: ChartKind;
    series: { name: string; data: number[] }[];
    categories: string[];
    height?: number;
    /** y ceiling; default is a round step just above the data */
    max?: number;
    /** dashed reference rule (e.g. average per day) */
    average?: number;
    averageLabel?: string;
    fill?: boolean;
    showY?: boolean;
    yFormat?: (value: number) => string;
    tooltip?: (index: number) => string;
    annotations?: Record<string, unknown>;
    color?: string;
  }>(),
  {
    kind: 'area',
    height: 232,
    fill: false,
    showY: true,
    averageLabel: 'avg',
    yFormat: (value: number) => (value >= 1000 ? `₹${Math.round(value / 1000)}k` : `₹${value}`)
  }
);

const emit = defineEmits<{ (e: 'select', index: number): void }>();

const dataMax = computed(() => Math.max(1, ...props.series.flatMap((s) => s.data)));

const yMax = computed(() => {
  if (props.max) return props.max;
  const step = dataMax.value > 20000 ? 10000 : dataMax.value > 5000 ? 5000 : 1000;
  return Math.max(step, Math.ceil(dataMax.value / step) * step);
});

const options = computed<Record<string, unknown>>(() => {
  const dark = themeResolved.value === 'dark';
  const line = props.color ?? palette.accent;

  const annotations: Record<string, unknown> = {
    ...(props.annotations ?? {})
  };
  if (props.average && props.average > 0) {
    annotations.yaxis = [
      ...((annotations.yaxis as unknown[]) ?? []),
      {
        y: props.average,
        borderColor: palette.inkMute,
        strokeDashArray: 4,
        label: {
          text: props.averageLabel,
          position: 'right',
          style: {
            background: 'transparent',
            color: palette.inkMute,
            fontSize: '9px'
          }
        }
      }
    ];
  }

  return {
    chart: {
      type: props.kind,
      toolbar: { show: false },
      background: 'transparent',
      fontFamily: 'IBM Plex Sans, system-ui, sans-serif',
      parentHeightOffset: 0,
      animations: { speed: 180 },
      events: {
        dataPointSelection: (_event: unknown, _context: unknown, config: { dataPointIndex: number }) => {
          emit('select', config.dataPointIndex);
        }
      }
    },
    theme: { mode: dark ? 'dark' : 'light' },
    colors: [line],
    stroke: { curve: 'smooth', width: 2 },
    markers: {
      size: 3,
      strokeWidth: 1.5,
      colors: [palette.page],
      strokeColors: [line],
      hover: { size: 5 }
    },
    dataLabels: { enabled: false },
    legend: { show: false },
    fill:
      props.kind === 'area'
        ? {
            type: 'gradient',
            gradient: {
              shadeIntensity: 0,
              opacityFrom: 0.28,
              opacityTo: 0,
              stops: [0, 100]
            }
          }
        : { type: 'solid', opacity: 1 },
    grid: {
      borderColor: palette.rule,
      strokeDashArray: 0,
      xaxis: { lines: { show: false } },
      yaxis: { lines: { show: props.showY } },
      padding: { left: 4, right: 8, top: 0, bottom: 0 }
    },
    xaxis: {
      categories: props.categories,
      axisBorder: { show: false },
      axisTicks: { show: false },
      labels: {
        rotate: 0,
        rotateAlways: false,
        hideOverlappingLabels: true,
        trim: false,
        style: { fontSize: '9.5px', colors: palette.inkMute }
      },
      tooltip: { enabled: false }
    },
    yaxis: {
      min: 0,
      max: yMax.value,
      tickAmount: 4,
      labels: {
        show: props.showY,
        style: {
          fontSize: '9px',
          colors: palette.inkMute,
          fontFamily: 'ui-monospace, Consolas, monospace'
        },
        formatter: (value: number) => props.yFormat(value)
      }
    },
    tooltip: {
      theme: dark ? 'dark' : 'light',
      custom: props.tooltip ? (ctx: { dataPointIndex: number }) => props.tooltip!(ctx.dataPointIndex) : undefined
    },
    annotations
  };
});
</script>

<template>
  <div class="relative w-full" :style="{ height: `${height}px` }">
    <AppApexChart :type="kind" :series="series" :options="options" :height="height" fill />
  </div>
</template>
