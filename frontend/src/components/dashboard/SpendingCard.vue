<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue';
import type { DashboardMonth } from '@/lib/api/client';
import { formatAmount } from '@/lib/utils/format';
import { palette } from '@/lib/chartPalette';
import AppStat from '@/components/ui/AppStat.vue';

// ApexCharts is ~500 kB: it loads with the chart, never with the dashboard shell.
const AppChart = defineAsyncComponent(() => import('@/components/ui/AppChart.vue'));

const props = defineProps<{ month: DashboardMonth }>();

const money = (value: number) => formatAmount(Math.round(value));
const buckets = computed(() => props.month.buckets ?? []);
const values = computed(() => buckets.value.map((b) => Number(b.amount.toFixed(2))));
const peakIndex = computed(() => buckets.value.findIndex((b) => b.key === props.month.peakBucket?.key));

// Labels at a readable interval, plus the peak day so its marker is anchored to
// a visible category. Every other point keeps its tooltip.
const categories = computed(() => {
  const n = buckets.value.length;
  const step = n > 12 ? 3 : n > 8 ? 2 : 1;
  const shown = new Set<number>([0, n - 1]);
  for (let i = 0; i < n; i++) {
    if (step === 3 ? i % 3 === 2 : i % step === 0) shown.add(i);
  }
  const peak = peakIndex.value;
  if (peak >= 0 && !shown.has(peak)) {
    if (shown.has(peak - 1)) shown.delete(peak - 1);
    else if (shown.has(peak + 1)) shown.delete(peak + 1);
    shown.add(peak);
  }
  return buckets.value.map((b, i) => (shown.has(i) ? String(Number(b.key.slice(-2))) : ''));
});

const yMax = computed(() => {
  const max = Math.max(...values.value, 1);
  return max <= 10000 ? 10000 : Math.ceil(max / 5000) * 5000;
});

const series = computed(() => [{ name: 'Spent', data: values.value }]);

const annotations = computed<Record<string, unknown>>(() => {
  const peak = props.month.peakBucket;
  if (!peak) return {};
  return {
    points: [
      {
        x: String(Number(peak.key.slice(-2))),
        y: peak.amount,
        marker: { size: 5, fillColor: palette.accent, strokeColor: palette.page, strokeWidth: 2 },
        label: {
          text: `${peak.label} · ${money(peak.amount)}`,
          borderWidth: 0,
          offsetY: -10,
          style: { background: palette.page, color: palette.ink, fontSize: '9.5px' }
        }
      }
    ]
  };
});

const tooltip = (index: number) => {
  const bucket = buckets.value[index];
  if (!bucket) return '';
  return `<div class="num" style="font-family:ui-monospace,Consolas,monospace;font-size:11px;color:${palette.ink};background:${palette.page};border:1px solid ${palette.rule};border-radius:8px;padding:7px 9px">
    <div style="color:${palette.inkMute};font-size:9.5px">${bucket.label}</div>
    <div style="margin-top:2px">${money(bucket.amount)}</div>
  </div>`;
};

const changeNote = computed(() => {
  const m = props.month;
  if (m.previousExpenses <= 0) return 'first month on record';
  const pct = Math.abs(m.expenseChangePct).toFixed(1);
  if (m.expenseChangePct < -0.05) return `${pct}% less than ${m.previousLabel}`;
  if (m.expenseChangePct > 0.05) return `${pct}% more than ${m.previousLabel}`;
  return `in line with ${m.previousLabel}`;
});
</script>

<template>
  <section
    class="card overflow-hidden rounded-card border border-rule bg-page p-4 md:p-5"
    aria-labelledby="spending-title"
  >
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div class="min-w-0">
        <p id="spending-title" class="text-label font-medium uppercase tracking-[0.1em] text-ink-mute">
          Spent this month
        </p>
        <p class="num mt-1.5 text-[27px] leading-none text-ink md:text-[30px]">{{ money(month.expenses) }}</p>
        <p class="mt-1.5 text-[11.5px] text-ink-mute">
          <span :class="month.expenseChangePct <= 0 ? 'text-credit' : 'text-ink-soft'">{{ changeNote }}</span>
          · {{ buckets.length }} of {{ new Date(month.to + 'T00:00:00').getDate() }} days
        </p>
      </div>
      <router-link to="/accounts/spending" class="link link-hover pt-1 text-[11.5px] font-medium text-accent"
        >Details →</router-link
      >
    </div>

    <div v-if="buckets.length" class="mt-3">
      <AppChart
        kind="area"
        :series="series"
        :categories="categories"
        :max="yMax"
        :average="month.averagePerDay"
        average-label="avg"
        :annotations="annotations"
        :tooltip="tooltip"
        :height="232"
      />
    </div>
    <div v-else class="flex min-h-[150px] flex-col items-center justify-center gap-1.5">
      <p class="text-[13px] text-ink">Nothing spent yet this month.</p>
      <router-link to="/accounts/spending" class="link link-hover text-[12px] font-medium text-accent"
        >Open the ledger →</router-link
      >
    </div>

    <div
      class="mt-4 grid grid-cols-2 gap-4 border-t border-rule-soft pt-4 sm:grid-cols-4 sm:gap-0 sm:[&>*+*]:border-l sm:[&>*+*]:border-rule-soft sm:[&>*+*]:pl-4"
    >
      <AppStat label="Average day" :value="money(month.averagePerDay)" size="sm" />
      <AppStat
        label="Peak day"
        :value="month.peakBucket ? money(month.peakBucket.amount) : '—'"
        :note="month.peakBucket?.label ?? ''"
        size="sm"
      />
      <AppStat
        label="Largest"
        :value="month.topCategory ? money(month.topCategory.amount) : '—'"
        :note="month.topCategory?.name ?? ''"
        size="sm"
      />
      <AppStat
        label="vs previous"
        :value="`${month.expenseChangePct > 0 ? '+' : ''}${month.expenseChangePct.toFixed(1)}%`"
        :tone="month.expenseChangePct > 0 ? 'debit' : 'credit'"
        size="sm"
      />
    </div>
  </section>
</template>
