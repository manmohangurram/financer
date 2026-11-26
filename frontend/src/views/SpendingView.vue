<script setup lang="ts">
import { ref, watch, computed, onMounted, onBeforeUnmount, defineAsyncComponent } from 'vue';
import { accounts, categories, analytics, transactions } from '@/lib/api/client';
import { useAccountsStore } from '@/lib/stores/accounts';
import { defaultSpendingRange, toLocalDateString, formatAmount } from '@/lib/utils/format';
import { budgetColor } from '@/lib/utils/budget';
import { resolveSpan, formatSpan, spanGranularity, type DateSpan } from '@/lib/utils/spendingRange';
import AppFolio from '@/components/ui/AppFolio.vue';
import AppEmpty from '@/components/ui/AppEmpty.vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppPageHeader from '@/components/ui/AppPageHeader.vue';
import AppMoneyBand from '@/components/ui/AppMoneyBand.vue';
import AppPager from '@/components/ui/AppPager.vue';
import AppMenu from '@/components/ui/AppMenu.vue';
import AppTabs from '@/components/ui/AppTabs.vue';
import AppSelect from '@/components/AppSelect.vue';
import AppDateRange from '@/components/ui/AppDateRange.vue';
import AccountFilterBar from '@/components/accounts/AccountFilterBar.vue';
import TransactionTable from '@/components/workspace/TransactionTable.vue';
// ApexCharts is ~300 kB of the bundle: load it with the chart, on this page only.
const SpendingChart = defineAsyncComponent(() => import('@/components/ui/AppChart.vue'));
import CategoryShareBar from '@/components/ui/CategoryShareBar.vue';

const store = useAccountsStore();

const accountList = ref<any[]>([]);
const categoryList = ref<any[]>([]);
const loading = ref(true);
const failed = ref(false);

const range = ref<'7D' | '1M' | '6M' | '1Y' | 'CUSTOM'>('7D');
const customStart = ref(toLocalDateString(new Date(Date.now() - 29 * 86400000)));
const customEnd = ref(toLocalDateString(new Date()));

const buckets = ref<any[]>([]);
const spendingCats = ref<any[]>([]);
const monthCats = ref<any[]>([]);
const monthBuckets = ref<any[]>([]);
const selectedKey = ref<string | null>(null);
const drillTxns = ref<any[]>([]);
const drillTotal = ref(0);
const drillPage = ref(0);
type SortKey = 'debit-desc' | 'debit-asc' | 'credit-desc' | 'credit-asc';
const drillSort = ref<SortKey>('debit-desc');
const DRILL_PAGE_SIZE = 10;

const SORTS = [
  { key: 'debit-desc', label: 'Amount ↓' },
  { key: 'debit-asc', label: 'Amount ↑' },
  { key: 'credit-desc', label: 'Refunds ↓' },
  { key: 'credit-asc', label: 'Refunds ↑' }
];
const sortLabel = computed(() => SORTS.find((o) => o.key === drillSort.value)?.label ?? 'Sort');

const RANGE_ITEMS = [
  { key: '7D', label: '7D' },
  { key: '1M', label: '1M' },
  { key: '6M', label: '6M' },
  { key: '1Y', label: '1Y' }
];

const signed = (t: any) => (t.type === 'DEBIT' ? -Math.abs(t.amount || 0) : Math.abs(t.amount || 0));
const pageSum = computed(() => drillTxns.value.reduce((s: number, t: any) => s + signed(t), 0));
const totalPages = computed(() => Math.max(1, Math.ceil(drillTotal.value / DRILL_PAGE_SIZE)));
const selectedLabel = computed(() => chartBuckets.value.find((b) => b.key === selectedKey.value)?.label ?? '');
/** API keys are `2026-11-10` or `2026-11`: the axis wants `10 Nov` / `Nov 26`. */
function bucketLabel(key: string): string {
  if (/^\d{4}-\d{2}-\d{2}$/.test(key)) {
    return new Date(`${key}T00:00:00`).toLocaleDateString(undefined, {
      day: 'numeric',
      month: 'short'
    });
  }
  if (/^\d{4}-\d{2}$/.test(key)) {
    return new Date(`${key}-01T00:00:00`).toLocaleDateString(undefined, {
      month: 'short',
      year: '2-digit'
    });
  }
  return key;
}

const chartBuckets = computed(() =>
  buckets.value.map((b: any) => ({
    key: b.key,
    label: bucketLabel(b.label || b.key),
    amount: b.amount || 0
  }))
);
const chartSeries = computed(() => [
  { name: 'Spent', data: chartBuckets.value.map((b) => Number(b.amount.toFixed(2))) }
]);
const chartCategories = computed(() => chartBuckets.value.map((b) => b.label));

function onChartSelect(index: number) {
  const point = chartBuckets.value[index];
  if (point) void selectBucket(point.key);
}

/** `8–14 Nov 2026` for the header, from the active range. */
/** The dates the page is showing, and how they read. */
const span = computed(() => resolveSpan(range.value, { from: customStart.value, to: customEnd.value }));
const rangeText = computed(() => formatSpan(span.value));
const resolvedRange = span;
const granularity = computed(() => spanGranularity(span.value));
const categoryBars = computed(() =>
  [...spendingCats.value]
    .map((c: any) => ({
      key: c.id,
      label: c.name,
      amount: Math.abs(c.debit || 0)
    }))
    .filter((c) => c.amount > 0)
    .sort((a, b) => b.amount - a.amount)
);
/** The stacked bar and its legend: top 6 by spend, everything else folded into Others. */
const composition = computed(() => {
  const head = categoryBars.value.slice(0, 6).map((c) => ({ name: c.label, amount: c.amount }));
  const rest = categoryBars.value.slice(6).reduce((sum, c) => sum + c.amount, 0);
  return rest > 0 ? [...head, { name: 'Others', amount: rest }] : head;
});

/** Budgets are monthly, so their progress always uses the current calendar month,
 *  independent of the 7D/1M/6M range that drives the chart and ledger. */
const monthSpendById = computed<Record<string, number>>(() => {
  const out: Record<string, number> = {};
  for (const c of monthCats.value) out[c.id] = Math.abs(c.debit || 0);
  return out;
});
const budgetedCats = computed(() =>
  categoryList.value
    .filter((c: any) => (c.monthlyBudget || 0) > 0)
    .map((c: any) => {
      const budget = c.monthlyBudget || 0;
      const spent = monthSpendById.value[c.id] || 0;
      return {
        id: c.id,
        name: c.name,
        budget,
        spent,
        left: budget - spent,
        over: spent > budget,
        used: budget > 0 ? Math.min(100, (spent / budget) * 100) : 0
      };
    })
    .sort((a, b) => b.used - a.used)
);
const unbudgetedCats = computed(() => {
  const budgeted = new Set(budgetedCats.value.map((b) => b.id));
  return monthCats.value
    .filter((c: any) => !budgeted.has(c.id) && Math.abs(c.debit || 0) > 0)
    .map((c: any) => ({
      id: c.id,
      name: c.name,
      spent: Math.abs(c.debit || 0)
    }))
    .sort((a, b) => b.spent - a.spent);
});
const budgetTotals = computed(() => {
  const planned = budgetedCats.value.reduce((s, c) => s + c.budget, 0);
  const spent = budgetedCats.value.reduce((s, c) => s + c.spent, 0);
  return {
    planned,
    spent,
    left: planned - spent,
    over: spent > planned,
    used: planned > 0 ? Math.min(100, (spent / planned) * 100) : 0
  };
});
const hasBudgets = computed(() => budgetedCats.value.length > 0);
const monthLabel = computed(() => new Date().toLocaleDateString(undefined, { month: 'long' }));

// Where the month stands: how far the plan is used, how far the month has run,
// and the two facts that describe the month's spending without a chart.
// `now` is a ref, refreshed when the tab comes back: a page left open overnight
// must not keep yesterday's "days left".
const now = ref(new Date());
const syncNow = () => {
  if (document.visibilityState === 'visible') now.value = new Date();
};

const monthProgress = computed(() => {
  const days = new Date(now.value.getFullYear(), now.value.getMonth() + 1, 0).getDate();
  return Math.round((now.value.getDate() / days) * 100);
});
const daysLeft = computed(() => {
  const days = new Date(now.value.getFullYear(), now.value.getMonth() + 1, 0).getDate();
  return Math.max(0, days - now.value.getDate());
});
const monthAverage = computed(() => budgetTotals.value.spent / Math.max(1, now.value.getDate()));
const monthPeak = computed(() =>
  monthBuckets.value.reduce<any>((best, b) => ((b.amount ?? 0) > (best?.amount ?? 0) ? b : best), null)
);

// What the band says under the headline figure.
const bandFacts = computed(() => [
  { label: 'Spent', value: formatAmount(budgetTotals.value.spent) },
  { label: 'Planned', value: formatAmount(budgetTotals.value.planned) },
  { label: 'Average / day', value: formatAmount(monthAverage.value) },
  {
    label: 'Peak day',
    value: monthPeak.value ? `${bucketLabel(monthPeak.value.key)} ${formatAmount(monthPeak.value.amount)}` : '—'
  }
]);
const paceBar = computed(() => ({
  percent: budgetTotals.value.used,
  color: budgetColor(budgetTotals.value.used),
  tickPercent: monthProgress.value
}));
const paceNote = computed(
  () =>
    `${Math.round(budgetTotals.value.used)}% of the plan · ${monthProgress.value}% through ${monthLabel.value} · ${daysLeft.value} ${daysLeft.value === 1 ? 'day' : 'days'} left`
);

function everyKey(from: string, to: string, word: 'day' | 'month'): string[] {
  const keys: string[] = [];
  const day = new Date(`${from}T00:00:00`);
  const end = new Date(`${to}T00:00:00`);
  if (word === 'day') {
    for (let d = day; d <= end; d = new Date(d.getTime() + 86400000)) keys.push(toLocalDateString(d));
  } else {
    for (
      let d = new Date(day.getFullYear(), day.getMonth(), 1);
      d <= end;
      d = new Date(d.getFullYear(), d.getMonth() + 1, 1)
    ) {
      keys.push(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`);
    }
  }
  return keys;
}

/** Days with no spending are zero, not absent: the line must not skip them. */
function fillGaps(raw: any[]): any[] {
  const word = granularity.value;
  // Always the whole period: selecting a bucket narrows the ledger, not the chart.
  const r = span.value;
  const byKey = new Map(raw.map((b: any) => [b.key, b]));
  const keys = raw.length ? raw.map((b: any) => b.key) : everyKey(r.from, r.to, word);
  const full = everyKey(r.from, r.to, word);
  const all = full.length ? full : keys;
  return all.map((key: string) => {
    const found = byKey.get(key);
    return { key, label: found?.label ?? key, amount: found?.amount ?? 0 };
  });
}

async function loadSpending() {
  const q: Record<string, unknown> = {
    range: range.value,
    accountId: store.state.selectedAccountId || undefined,
    ...rangeParams()
  };
  const resp = await analytics().spending(q);
  buckets.value = fillGaps(resp.buckets || []);
  spendingCats.value = resp.categories || [];
}

// The calendar month behind the budget card: budgets are monthly, so this card
// always looks at the current month, whatever the period tabs say — but it does
// follow the account chips, like every other figure on the page.
async function loadMonth() {
  const now = new Date();
  const from = new Date(now.getFullYear(), now.getMonth(), 1);
  const to = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  const resp = await analytics().spending({
    range: 'CUSTOM',
    from: toLocalDateString(from),
    to: toLocalDateString(to),
    accountId: store.state.selectedAccountId || undefined
  });
  monthCats.value = resp.categories || [];
  monthBuckets.value = resp.buckets || [];
}

function rangeParams(): { from?: string; to?: string } {
  if (range.value === 'CUSTOM') return { from: span.value.from, to: span.value.to };
  return {};
}

function selectedRange(): DateSpan | null {
  if (!selectedKey.value) return null;
  if (granularity.value === 'day') return { from: selectedKey.value, to: selectedKey.value };
  const [y, m] = selectedKey.value.split('-').map(Number);
  const last = new Date(y, m, 0).getDate();
  return {
    from: `${selectedKey.value}-01`,
    to: `${selectedKey.value}-${String(last).padStart(2, '0')}`
  };
}

async function loadDrillPage(page: number) {
  const r = selectedRange() || span.value;
  const [sortBy, sortDir] = drillSort.value.split('-');
  const resp = await transactions().listTransactions({
    pageSize: DRILL_PAGE_SIZE,
    offset: page * DRILL_PAGE_SIZE,
    sortBy,
    sortDir,
    dateFrom: r.from,
    dateTo: r.to,
    accountId: store.state.selectedAccountId || undefined
  });
  drillTxns.value = resp.transactions || [];
  drillTotal.value = resp.totalCount || drillTxns.value.length;
}

async function refresh() {
  failed.value = false;
  drillPage.value = 0;
  try {
    await Promise.all([loadSpending(), loadMonth(), loadDrillPage(0)]);
  } catch {
    failed.value = true;
  }
}

async function selectBucket(key: string) {
  selectedKey.value = selectedKey.value === key ? null : key;
  await refresh();
}

/** A hand-picked range replaces the preset and drives the whole page. */
function applyCustomRange(picked: { from: string; to: string }) {
  customStart.value = picked.from;
  customEnd.value = picked.to;
  range.value = 'CUSTOM';
}

async function clearBucket() {
  selectedKey.value = null;
  await refresh();
}

async function goPage(p: number) {
  if (p < 0 || p >= totalPages.value) return;
  drillPage.value = p;
  await loadDrillPage(p);
}

/** The sort watcher below does the reload, so this only records the choice. */
function setSort(key: string) {
  drillSort.value = key as SortKey;
}

// One refresh per tick, however many filters moved: range, dates and sort all
// funnel here, so a single Apply is a single round of requests.
let refreshTimer: ReturnType<typeof setTimeout> | null = null;
function scheduleRefresh() {
  if (refreshTimer) clearTimeout(refreshTimer);
  refreshTimer = setTimeout(() => {
    refreshTimer = null;
    selectedKey.value = null;
    void refresh();
  }, 0);
}

watch(range, scheduleRefresh);
watch([customStart, customEnd], () => {
  if (customStart.value && customEnd.value && customStart.value > customEnd.value) {
    customEnd.value = customStart.value;
    return;
  }
  if (range.value === 'CUSTOM') scheduleRefresh();
});
watch(
  () => store.state.selectedAccountId,
  async () => {
    selectedKey.value = null;
    await refresh();
  }
);
watch(drillSort, refresh);

async function pickDefaultRange() {
  try {
    const resp = await transactions().listTransactions({ pageSize: 1 });
    const latest = resp?.transactions?.[0]?.occurredAt;
    if (!latest) return;
    const picked = defaultSpendingRange(latest);
    range.value = picked.range;
    if (picked.start && picked.end) {
      customStart.value = picked.start;
      customEnd.value = picked.end;
    }
  } catch {
    /* the default range is good enough if this fails */
  }
}

async function loadAll() {
  loading.value = true;
  try {
    const [accResp, catResp] = await Promise.all([
      accounts().listAccounts({}),
      categories().listCategories({ pageSize: 10000 })
    ]);
    accountList.value = accResp.accounts || [];
    categoryList.value = catResp.categories || [];
    await pickDefaultRange();
    await refresh();
  } catch {
    failed.value = true;
  }
  loading.value = false;
}

onMounted(loadAll);
onMounted(() => document.addEventListener('visibilitychange', syncNow));
onBeforeUnmount(() => document.removeEventListener('visibilitychange', syncNow));
</script>

<template>
  <div class="flex w-full flex-col gap-3">
    <AppPageHeader eyebrow="Analysis" title="Spending">
      <AppTabs v-model="range" :items="RANGE_ITEMS" />
      <span class="h-4 w-px shrink-0 bg-rule" aria-hidden="true"></span>
      <AppDateRange
        :from="resolvedRange.from"
        :to="resolvedRange.to"
        :label="rangeText"
        :active="range === 'CUSTOM'"
        @apply="applyCustomRange"
      />
    </AppPageHeader>

    <!-- The account filter only: the same chip strip as the Accounts page, with
         the period living up in the header. Chips slide sideways when they
         overflow; narrow screens trade them for the native picker. -->
    <section class="rounded-card border border-rule bg-page px-3 py-1.5 md:px-4">
      <div class="hidden md:block">
        <AccountFilterBar :accounts="accountList" :bordered="false" justify="start" />
      </div>
      <div class="py-1.5 md:hidden">
        <AppSelect
          :model-value="store.state.selectedAccountId || ''"
          label="Account"
          @update:model-value="(v?: string | number) => store.select(String(v ?? '') || null)"
        >
          <option value="">All accounts</option>
          <option v-for="a in accountList" :key="a.id" :value="a.id">
            {{ a.nickname || a.bankName }}
          </option>
        </AppSelect>
      </div>
    </section>

    <p
      v-if="failed"
      class="flex shrink-0 items-center justify-between rounded-card border border-debit/40 px-3.5 py-2.5 text-[12.5px] text-debit"
    >
      Spending could not be loaded.
      <AppButton variant="quiet" size="sm" @click="refresh">Retry</AppButton>
    </p>

    <AppStamp v-if="selectedKey" tone="accent" class="cursor-pointer self-start" @click="clearBucket">
      {{ selectedLabel }} ✕
    </AppStamp>

    <!-- The month's plan leads: what is left, what it is made of, and whether
           the month is running ahead of it. The period tabs govern the chart and
           the ledger below, not this band. -->
    <AppMoneyBand
      :label="hasBudgets ? `Left to spend · ${monthLabel}` : 'No monthly budget'"
      :value="hasBudgets ? `${budgetTotals.over ? '−' : ''}${formatAmount(Math.abs(budgetTotals.left))}` : ''"
      :value-tone="budgetTotals.over ? 'debit' : 'ink'"
      :facts="bandFacts"
      :bar="hasBudgets ? paceBar : null"
      :note="hasBudgets ? paceNote : ''"
    >
      <template v-if="!hasBudgets" #under>
        <router-link to="/accounts/categories" class="mt-2 inline-block text-[13px] font-medium text-accent">
          Set monthly limits →
        </router-link>
      </template>
    </AppMoneyBand>

    <div class="grid grid-cols-1 gap-3 lg:grid-cols-[7fr_3fr]">
      <!-- when the money went: four fifths of the row -->
      <AppFolio
        title="Spending over time"
        subtitle="click a day to narrow the ledger"
        class="lg:col-span-1"
        body-class="flex flex-col justify-center"
      >
        <SpendingChart
          v-if="!loading && chartBuckets.length"
          kind="line"
          :series="chartSeries"
          :categories="chartCategories"
          :height="300"
          @select="onChartSelect"
        />
        <span v-else-if="loading" class="block h-[300px] animate-pulse rounded-control bg-page-sunk" />
        <AppEmpty v-else title="Nothing in this range." hint="Try a wider period, or clear the account filter." />
      </AppFolio>

      <!-- what it went on: monthly budgets take precedence, otherwise the range share -->
      <AppFolio
        :title="hasBudgets ? 'Budget by category' : 'Where it went'"
        :subtitle="hasBudgets ? 'vs monthly budget' : 'top 6 by spend'"
        class="lg:col-span-1"
        body-class="flex flex-col"
        flush
      >
        <div v-if="hasBudgets" class="flex max-h-[330px] flex-1 flex-col overflow-auto">
          <div
            v-for="c in budgetedCats"
            :key="c.id"
            class="border-b border-rule-soft px-3 py-2 last:border-0"
            :class="c.over ? 'bg-debit/5' : ''"
          >
            <div class="flex items-center gap-2">
              <span
                class="inline-flex min-w-0 items-center gap-1.5 rounded-full border border-rule-soft bg-page-sunk/70 py-0.5 pl-1.5 pr-2.5 text-[11.5px] text-ink-soft"
              >
                <span
                  class="h-[6px] w-[6px] shrink-0 rounded-full"
                  :style="{ background: budgetColor(c.used) }"
                  aria-hidden="true"
                />
                <span class="truncate">{{ c.name }}</span>
              </span>
              <span class="num ml-auto shrink-0 text-[11px] text-ink-mute"
                >{{ formatAmount(c.spent) }} / {{ formatAmount(c.budget) }}</span
              >
            </div>
            <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-page-sunk">
              <div
                class="h-full rounded-full"
                :style="{ width: `${Math.max(c.used, 1.5)}%`, background: budgetColor(c.used) }"
              ></div>
            </div>
          </div>
          <div
            v-for="c in unbudgetedCats"
            :key="c.id"
            class="flex items-center gap-2 border-b border-rule-soft px-3 py-2 last:border-0"
          >
            <span
              class="inline-flex min-w-0 items-center gap-1.5 rounded-full border border-rule-soft bg-page-sunk/70 py-0.5 pl-1.5 pr-2.5 text-[11.5px] text-ink-soft"
            >
              <span class="h-[6px] w-[6px] shrink-0 rounded-full bg-ink-faint" aria-hidden="true" />
              <span class="truncate">{{ c.name }}</span>
            </span>
            <span class="num ml-auto shrink-0 text-[11px] text-ink">{{ formatAmount(c.spent) }}</span>
          </div>
        </div>

        <div v-else-if="composition.length" class="flex flex-1 flex-col">
          <div class="px-4 pt-4">
            <CategoryShareBar :items="composition" :height="30" />
          </div>
          <div class="mt-6 flex flex-1 flex-col gap-0 px-4 pb-2">
            <div
              v-for="(c, i) in composition"
              :key="c.name"
              class="flex items-center gap-2 border-b border-rule-soft py-[7px] last:border-0"
            >
              <span
                class="inline-flex min-w-0 items-center gap-1.5 rounded-full border border-rule-soft bg-page-sunk/70 py-0.5 pl-1.5 pr-2.5 text-[11.5px] text-ink-soft"
              >
                <span
                  class="h-[6px] w-[6px] shrink-0 rounded-full"
                  :style="{
                    background: `var(--chart-${Math.min(i, 7)})`
                  }"
                  aria-hidden="true"
                />
                <span class="truncate">{{ c.name }}</span>
              </span>
              <span class="num ml-auto text-[12px] font-medium text-ink">{{ formatAmount(c.amount) }}</span>
            </div>
          </div>
        </div>
        <span v-else-if="loading" class="m-4 block h-[140px] animate-pulse rounded-control bg-page-sunk" />
        <div v-else class="px-4 py-6">
          <AppEmpty title="No categorised spending." hint="Categories appear here once entries carry one." />
        </div>
      </AppFolio>

      <!-- the entries themselves: the sort lives in the card head, so the table
           starts immediately under it -->
      <AppFolio title="Ledger" flush class="lg:col-span-2" body-class="flex flex-col">
        <template #actions>
          <!-- Phone: one control, because four labels would eat the card's title.
               Desktop: the four, so the sort is one click away. -->
          <AppMenu class="sm:hidden" align="right" label="Sort ledger" :items="SORTS" @select="setSort">
            <template #trigger="{ toggle }">
              <button
                type="button"
                class="btn btn-ghost h-[30px] min-h-0 gap-1 rounded-control border border-rule px-2 text-[11px] font-medium text-ink-soft"
                @click="toggle"
              >
                {{ sortLabel }}
              </button>
            </template>
          </AppMenu>
          <div class="hidden items-center gap-1 sm:flex">
            <button
              v-for="opt in SORTS"
              :key="opt.key"
              type="button"
              class="btn btn-ghost h-[30px] min-h-0 rounded-control border px-2.5 text-[12px] font-medium"
              :class="
                drillSort === opt.key
                  ? 'border-accent/40 bg-accent-wash text-accent hover:bg-accent-wash'
                  : 'border-transparent text-ink-soft hover:bg-page-sunk hover:text-ink'
              "
              @click="setSort(opt.key)"
            >
              {{ opt.label }}
            </button>
          </div>
        </template>

        <div class="flex flex-col">
          <!-- Same table as the Accounts page: Date · Name · Categories · Amount. -->
          <div v-if="loading && !drillTxns.length" class="flex flex-col gap-2 p-4">
            <span v-for="i in 5" :key="i" class="block h-10 animate-pulse rounded-control bg-page-sunk" />
          </div>
          <TransactionTable
            v-else
            readonly
            :transactions="drillTxns"
            :categories="categoryList"
            empty-text="No entries in this range."
          />

          <!-- The carried-forward line closes the page: it sits at the bottom of
               the ledger even when the range has only a few rows. -->
          <div
            v-if="!loading && drillTxns.length"
            class="flex items-center border-t border-rule bg-page-sunk px-3 py-2.5"
          >
            <span class="flex-1 text-[12px] text-ink-mute">Carried forward · this page</span>
            <span class="num w-[140px] pr-3 text-right font-display text-[15px] text-ink">{{
              formatAmount(pageSum)
            }}</span>
          </div>
        </div>

        <template #footer>
          <AppPager :page="drillPage" :total-pages="totalPages" label="Spending ledger" @page-change="goPage" />
        </template>
      </AppFolio>
    </div>
  </div>
</template>
