<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { investments } from '@/lib/api/client';
import { formatCurrency, toLocalDateString } from '@/lib/utils/format';
import TimeRange from '@/components/TimeRange.vue';
import DatePicker from '@/components/DatePicker.vue';
import { RefreshCw } from '@lucide/vue';

const props = defineProps<{ investment: any }>();

// Stocks get an intraday 1D range; mutual funds start at 7D (same as Spending).
// Both share the preset ranges and support Custom dates (up to 5 years).
const CHART_RANGES = computed(() => {
  const base = [
    { id: '7D', label: '7D' },
    { id: '1M', label: '1M' },
    { id: '6M', label: '6M' },
    { id: '1Y', label: '1Y' },
    { id: '3Y', label: '3Y' },
    { id: 'CUSTOM', label: 'Custom' }
  ];
  if (props.investment?.investmentType === 'INVESTMENT_TYPE_STOCK') {
    base.unshift({ id: '1D', label: '1D' });
  }
  return base;
});
type RangeId = '1D' | '7D' | '1M' | '6M' | '1Y' | '3Y' | 'CUSTOM';

const range = ref<RangeId>('3Y');
const customStart = ref(toLocalDateString(new Date(Date.now() - 29 * 86400000)));
const customEnd = ref(toLocalDateString(new Date()));
const points = ref<{ t: number; close: number }[]>([]);
const loading = ref(false);
const refreshing = ref(false);
const error = ref('');
const hovered = ref<number | null>(null);

const W = 720;
const H = 220;
const PAD_L = 46;
const PAD_R = 10;
const PAD_T = 14;
const PAD_B = 6;

const up = computed(() => points.value.length > 1 && points.value[points.value.length - 1].close >= points.value[0].close);
const strokeCls = computed(() => (up.value ? 'stroke-income' : 'stroke-expense'));
const cssVar = computed(() => (up.value ? 'var(--color-income)' : 'var(--color-expense)'));

const min = computed(() => points.value.reduce((m, p) => Math.min(m, p.close), Infinity));
const max = computed(() => points.value.reduce((m, p) => Math.max(m, p.close), -Infinity));

const yTicks = computed(() => {
  const lo = min.value;
  const hi = Math.max(max.value, lo + 1);
  const lo15 = lo + (hi - lo) * 0.1;
  const out: number[] = [];
  for (let i = 0; i < 6; i++) out.push(lo15 + ((hi - lo15) * i) / 5);
  return out;
});

const xTicks = computed(() => {
  const n = points.value.length;
  if (n < 2) return [] as number[];
  const idx: number[] = [];
  for (let i = 0; i < 8; i++) {
    idx.push(Math.min(n - 1, 1 + Math.round(((n - 2) * i) / 7)));
  }
  return idx;
});

function yFor(v: number): number {
  const lo = min.value;
  const hi = Math.max(max.value, lo + 1);
  const spanY = H - PAD_T - PAD_B;
  return H - PAD_B - ((v - lo) / (hi - lo)) * spanY;
}

function trimZeros(s: string): string {
  return s.replace(/\.?0+$/, '');
}

function axisPrice(v: number): string {
  if (v >= 1000000) return `₹${trimZeros((v / 1000000).toFixed(2))}M`;
  if (v >= 1000) return `₹${trimZeros((v / 1000).toFixed(2))}K`;
  if (v >= 100) return `₹${Math.round(v)}`;
  return `₹${v.toFixed(1)}`;
}

const customIsMonth = computed(() => {
  const a = new Date(customStart.value).getTime();
  const b = new Date(customEnd.value).getTime();
  return b - a > 31 * 86400000;
});

function xLabel(t: number): string {
  const d = new Date(t * 1000);
  if (range.value === '1D') return d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
  if (range.value === 'CUSTOM' && customIsMonth.value) return d.toLocaleDateString('en-US', { month: 'short', year: '2-digit' });
  if (range.value === '6M' || range.value === '1Y' || range.value === '3Y') {
    return d.toLocaleDateString('en-US', { month: 'short', year: '2-digit' });
  }
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

const coords = computed(() => {
  const n = points.value.length;
  if (n === 0) return [] as { x: number; y: number }[];
  const lo = min.value;
  const hi = Math.max(max.value, lo + 1);
  const spanX = W - PAD_L - PAD_R;
  const spanY = H - PAD_T - PAD_B;
  return points.value.map((p, i) => ({
    x: PAD_L + (n === 1 ? spanX / 2 : (i / (n - 1)) * spanX),
    y: H - PAD_B - ((p.close - lo) / (hi - lo)) * spanY
  }));
});

function smoothPath(pts: { x: number; y: number }[]): string {
  if (pts.length < 2) return '';
  let d = `M ${pts[0].x.toFixed(2)} ${pts[0].y.toFixed(2)}`;
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i - 1] || pts[i];
    const p1 = pts[i];
    const p2 = pts[i + 1];
    const p3 = pts[i + 2] || p2;
    const c1x = p1.x + (p2.x - p0.x) / 6;
    const c1y = p1.y + (p2.y - p0.y) / 6;
    const c2x = p2.x - (p3.x - p1.x) / 6;
    const c2y = p2.y - (p3.y - p1.y) / 6;
    d += ` C ${c1x.toFixed(2)} ${c1y.toFixed(2)}, ${c2x.toFixed(2)} ${c2y.toFixed(2)}, ${p2.x.toFixed(2)} ${p2.y.toFixed(2)}`;
  }
  return d;
}

const linePath = computed(() => smoothPath(coords.value));

const areaPath = computed(() => {
  if (coords.value.length < 2) return '';
  const first = coords.value[0];
  const last = coords.value[coords.value.length - 1];
  const base = H - PAD_B;
  return `${linePath.value} L ${last.x.toFixed(2)} ${base} L ${first.x.toFixed(2)} ${base} Z`;
});

function onMove(e: MouseEvent) {
  const rect = (e.currentTarget as SVGSVGElement).getBoundingClientRect();
  const x = ((e.clientX - rect.left) / rect.width) * W;
  const n = points.value.length;
  if (n === 0) return;
  const spanX = W - PAD_L - PAD_R;
  const i = Math.round((x - PAD_L) / (spanX / Math.max(n - 1, 1)));
  hovered.value = Math.max(0, Math.min(n - 1, i));
}

function tickLabel(t: number): string {
  const d = new Date(t * 1000);
  if (range.value === '1D') return d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

const hoveredPoint = computed(() => (hovered.value === null ? null : points.value[hovered.value]));
const hoverX = computed(() => (hovered.value === null ? 0 : coords.value[hovered.value]?.x ?? 0));

function chartQuery() {
  const q: Record<string, string> = { id: props.investment.id };
  if (range.value === 'CUSTOM') {
    q.from = customStart.value;
    q.to = customEnd.value;
  } else {
    q.range = range.value.toLowerCase();
  }
  return q;
}

async function load() {
  if (!props.investment?.id || !props.investment?.symbol) return;
  loading.value = true;
  error.value = '';
  hovered.value = null;
  try {
    const resp = await investments().getPriceHistory(chartQuery());
    points.value = resp.points || [];
  } catch (e: any) {
    points.value = [];
    error.value = e?.message || 'Failed to load price history';
  }
  loading.value = false;
}

async function forceRefresh() {
  if (!props.investment?.id || !props.investment?.symbol) return;
  refreshing.value = true;
  error.value = '';
  hovered.value = null;
  try {
    const resp = await investments().getPriceHistory({ ...chartQuery(), refresh: '1' });
    points.value = resp.points || [];
  } catch (e: any) {
    error.value = e?.message || 'Failed to refresh price history';
  }
  refreshing.value = false;
}

// End date always stays toward the future (never before the start).
watch(customStart, (v) => {
  if (v && customEnd.value && v > customEnd.value) customEnd.value = v;
  if (range.value === 'CUSTOM') load();
});
watch(customEnd, (v) => {
  if (v && customStart.value && v < customStart.value) customStart.value = v;
  if (range.value === 'CUSTOM') load();
});

// Reset to a range the current investment type offers (e.g. MF has no 1D).
watch(() => props.investment?.id, () => {
  if (!CHART_RANGES.value.some((r) => r.id === range.value)) range.value = '3Y';
  load();
});
watch(range, load);
</script>

<template>
  <div class="rounded-xl border border-border bg-base-200 p-4">
    <div class="flex items-center justify-between mb-3">
      <h3 class="text-[15px] font-semibold text-text">Price History</h3>
      <div class="flex items-center gap-2">
        <TimeRange v-model="range" :options="CHART_RANGES" />
        <template v-if="range === 'CUSTOM'">
          <DatePicker v-model="customStart" class="input-sm w-32" />
          <span class="text-[12px] text-subtle">to</span>
          <DatePicker v-model="customEnd" class="input-sm w-32" />
        </template>
        <button
          class="w-7 h-7 rounded-lg border border-border text-subtle hover:text-primary-400 hover:border-primary-500/40 flex items-center justify-center transition-colors"
          :disabled="refreshing || loading"
          aria-label="Refresh price history"
          @click="forceRefresh"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="refreshing ? 'animate-spin' : ''" />
        </button>
      </div>
    </div>

    <p v-if="!props.investment?.symbol" class="text-center py-10 text-subtle text-[13px]">No price history for this investment.</p>
    <div v-else-if="loading" class="text-center py-10 text-subtle text-[13px]">Loading price history…</div>
    <div v-else-if="error" class="text-center py-10 text-expense text-[13px]">{{ error }}</div>
    <div v-else-if="points.length === 0" class="text-center py-10 text-subtle text-[13px]">No price data available.</div>

    <div v-else class="px-2 pb-5 pt-1">
      <div class="relative">
      <svg
        :viewBox="`0 0 ${W} ${H}`"
        class="w-full"
        role="img"
        :aria-label="`${props.investment.name} closing price`"
        @mousemove="onMove"
        @mouseleave="hovered = null"
      >
        <defs>
          <linearGradient :id="'area-' + props.investment.id" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" :stop-color="cssVar" stop-opacity="0.25" />
            <stop offset="100%" :stop-color="cssVar" stop-opacity="0.02" />
          </linearGradient>
        </defs>

        <g v-for="(tv, i) in yTicks" :key="'y' + i">
          <line :x1="PAD_L" :x2="W - PAD_R" :y1="yFor(tv)" :y2="yFor(tv)" class="stroke-border/15" stroke-width="1" />
        </g>

        <path :d="areaPath" :fill="`url(#area-${props.investment.id})`" />
        <path :d="linePath" fill="none" :class="strokeCls" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />

        <line
          v-if="hovered !== null"
          :x1="hoverX" :x2="hoverX" y1="0" :y2="H - PAD_B"
          class="stroke-track"
          stroke-width="1"
        />
        <circle
          v-if="hovered !== null"
          :cx="hoverX" :cy="coords[hovered]?.y ?? 0" r="4"
          :class="strokeCls"
          class="fill-base-200"
          stroke-width="2"
        />
      </svg>

      <div
        v-for="(tv, i) in yTicks"
        :key="'yl' + i"
        class="absolute text-[15px] leading-none text-subtle pointer-events-none"
        :style="{ left: (PAD_L / W) * 100 + '%', top: (yFor(tv) / H) * 100 + '%', transform: 'translate(calc(-100% - 10px), -50%)' }"
      >{{ axisPrice(tv) }}</div>

      <div
        v-for="(idx, i) in xTicks"
        :key="'xl' + i"
        class="absolute text-[15px] leading-none text-subtle pointer-events-none whitespace-nowrap"
        :style="{ left: (coords[idx].x / W) * 100 + '%', top: '100%', transform: 'translateX(-50%)' }"
      >{{ xLabel(points[idx].t) }}</div>

      <div
        v-if="hoveredPoint"
        class="absolute top-0 -translate-x-1/2 px-2 py-1 rounded-md bg-base-300 border border-border text-[11px] whitespace-nowrap pointer-events-none"
        :style="{ left: (hoverX / W) * 100 + '%' }"
      >
        <span class="text-subtle">{{ tickLabel(hoveredPoint.t) }}</span>
        <span class="ml-1.5 font-semibold" :class="up ? 'text-income' : 'text-expense'">{{ formatCurrency(hoveredPoint.close) }}</span>
      </div>
      </div>
    </div>
  </div>
</template>
