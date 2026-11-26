<script setup lang="ts">
import { computed } from 'vue';
import type { DashboardData } from '@/lib/api/client';
import { formatAmount } from '@/lib/utils/format';
import AppStat from '@/components/ui/AppStat.vue';

const props = defineProps<{ data: DashboardData }>();

const money = (value: number) => formatAmount(Math.round(value));
const signed = (value: number) => (value > 0 ? `+${money(value)}` : money(value));

const assets = computed(() => Math.max(0, props.data.cash) + Math.max(0, props.data.savings));
const owed = computed(() => Math.max(0, props.data.debt));
const assetTotal = computed(() => assets.value + owed.value);
const assetsPct = computed(() => (assetTotal.value > 0 ? (assets.value / assetTotal.value) * 100 : 0));
const owedPct = computed(() => 100 - assetsPct.value);

const asOf = computed(() =>
  new Date().toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
);

const changeNote = computed(() => {
  const m = props.data.month;
  if (m.previousExpenses <= 0) return 'booked this month';
  const pct = Math.abs(m.expenseChangePct).toFixed(1);
  if (m.expenseChangePct < -0.05) return `${pct}% less than ${m.previousLabel}`;
  if (m.expenseChangePct > 0.05) return `${pct}% more than ${m.previousLabel}`;
  return `in line with ${m.previousLabel}`;
});

const compositionLabel = computed(
  () =>
    `Assets ${money(assets.value)}, ${assetsPct.value.toFixed(1)} percent; owed ${money(owed.value)}, ${owedPct.value.toFixed(1)} percent`
);
</script>

<template>
  <section
    class="card flex flex-col overflow-hidden rounded-card border border-rule bg-page p-4 md:p-5"
    aria-label="Net worth"
  >
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="text-label font-medium uppercase tracking-[0.1em] text-ink-mute">Net worth</p>
        <p class="mt-1.5 truncate text-[11.5px] text-ink-mute">
          {{ data.accounts.length }} {{ data.accounts.length === 1 ? 'account' : 'accounts' }} · {{ asOf }}
        </p>
      </div>
      <span
        class="inline-flex shrink-0 items-center rounded-full border border-credit/30 bg-credit/10 px-2.5 py-1 font-mono text-[10.5px] leading-none text-credit"
      >
        {{ signed(data.month.net) }} this month
      </span>
    </div>

    <p class="num mt-4 text-[34px] leading-none text-ink md:text-[46px]">{{ money(data.totalBalance) }}</p>

    <div class="mt-5">
      <div
        class="flex h-2.5 gap-[3px] overflow-hidden rounded-full bg-page-sunk"
        role="img"
        :aria-label="compositionLabel"
      >
        <i class="block h-full rounded-full bg-accent" :style="{ width: `${assetsPct}%` }"></i>
        <i class="block h-full rounded-full bg-debit/85" :style="{ width: `${owedPct}%` }"></i>
      </div>
      <div class="mt-2.5 flex flex-wrap items-center gap-x-5 gap-y-1.5 text-[12px] text-ink-mute">
        <span class="flex items-center gap-2">
          <i class="h-2 w-2 shrink-0 rounded-full bg-accent"></i>
          <span class="num text-ink">{{ money(assets) }}</span> assets · {{ assetsPct.toFixed(1) }}%
        </span>
        <span class="flex items-center gap-2">
          <i class="h-2 w-2 shrink-0 rounded-full bg-debit/85"></i>
          <span class="num text-ink">{{ money(owed) }}</span> owed · {{ owedPct.toFixed(1) }}%
        </span>
      </div>
    </div>

    <div
      class="mt-5 grid grid-cols-1 gap-4 border-t border-rule-soft pt-4 sm:grid-cols-3 sm:gap-0 sm:[&>*+*]:border-l sm:[&>*+*]:border-rule-soft sm:[&>*+*]:pl-4"
    >
      <AppStat label="In" :value="signed(data.month.income)" note="booked this month" tone="credit" />
      <AppStat label="Out" :value="`−${money(data.month.expenses)}`" :note="changeNote" tone="debit" />
      <AppStat label="Net" :value="signed(data.month.net)" note="income − expenses" />
    </div>
  </section>
</template>
