<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import ApexCharts from 'apexcharts';
import { palette } from '@/lib/chartPalette';

// The daisyUI-charts pattern (ApexCharts) with the ledger palette plugged in:
// one wrapper owns mount/update/destroy so pages only pass options and series.
const props = defineProps<{
  type: string;
  series: unknown;
  options: Record<string, unknown>;
  height: number;
  /** Stretch to the host element's height instead of the fixed `height`. */
  fill?: boolean;
}>();

const host = ref<HTMLElement | null>(null);
let chart: ApexCharts | null = null;
let observer: ResizeObserver | null = null;

/** The host is absolutely positioned inside a fixed-height grid row, so measuring it
 *  cannot feed back into the layout: the chart follows the card, never the reverse. */
function currentHeight(): number {
  if (!props.fill) return props.height;
  return Math.max(120, Math.floor(host.value?.clientHeight ?? props.height));
}

function baseOptions() {
  return {
    chart: {
      height: currentHeight(),
      fontFamily: 'IBM Plex Sans, system-ui, sans-serif',
      foreColor: palette.inkMute,
      toolbar: { show: false },
      animations: { speed: 180, easing: 'easeout' },
      background: 'transparent'
    },
    theme: { mode: 'light' as const },
    grid: { borderColor: palette.rule, strokeDashArray: 0, padding: { left: 6, right: 6 } },
    tooltip: { theme: 'light' as const, style: { fontFamily: 'IBM Plex Sans, system-ui, sans-serif' } }
  };
}

function render() {
  if (!host.value) return;
  const base = baseOptions();
  const merged = { ...base, ...props.options, chart: { ...base.chart, ...(props.options.chart as object) } } as Record<
    string,
    unknown
  > & { chart: Record<string, unknown> };
  // ApexCharts reads the type from `chart.type`; a top-level `type` is ignored.
  merged.chart.type = merged.chart.type ?? props.type;
  if (!chart) {
    chart = new ApexCharts(host.value, { ...merged, series: props.series } as never);
    void chart.render();
  } else {
    void chart.updateOptions({ ...merged, series: props.series } as never, false, false);
  }
}

onMounted(() => {
  render();
  if (props.fill && host.value) {
    // A percentage height re-resolves on layout: nudge Apex when the card resizes.
    observer = new ResizeObserver(() =>
      chart?.updateOptions({ chart: { height: currentHeight() } } as never, false, false)
    );
    observer.observe(host.value);
  }
});

// Series changes on every filter/period change; theme flips re-read the palette.
watch(() => [props.series, props.options], render, { deep: true });
watch(palette, render, { deep: true });

onBeforeUnmount(() => {
  observer?.disconnect();
  chart?.destroy();
  chart = null;
});
</script>

<template>
  <div ref="host" :class="fill ? 'absolute inset-0' : 'w-full'" />
</template>
