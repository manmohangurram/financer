<script setup lang="ts">
import { ref, watch, computed, onMounted, defineAsyncComponent } from 'vue';
import { accounts, categories, analytics, transactions } from '@/lib/api/client';
import { useAccountsStore } from '@/lib/stores/accounts';
import { defaultSpendingRange, toLocalDateString, formatCurrency } from '@/lib/utils/format';
import AppFolio from '@/components/ui/AppFolio.vue';
import AppTable from '@/components/ui/AppTable.vue';
import AppEmpty from '@/components/ui/AppEmpty.vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppTabs from '@/components/ui/AppTabs.vue';
import AppSelect from '@/components/ui/AppSelect.vue';
import AppField from '@/components/ui/AppField.vue';
// ApexCharts is ~300 kB of the bundle: load it with the charts, on this page only.
const SpendingColumnsChart = defineAsyncComponent(() => import('@/components/ui/SpendingColumnsChart.vue'));
import CategoryShareBar from '@/components/ui/CategoryShareBar.vue';
import AppStamp from '@/components/ui/AppStamp.vue';
import AddTransactionModal from '@/components/workspace/AddTransactionModal.vue';

const store = useAccountsStore();

const accountList = ref<any[]>([]);
const categoryList = ref<any[]>([]);
const loading = ref(true);
const failed = ref(false);
const adding = ref(false);

const range = ref<'7D' | '1M' | '6M' | '1Y' | 'CUSTOM'>('7D');
const customStart = ref(toLocalDateString(new Date(Date.now() - 29 * 86400000)));
const customEnd = ref(toLocalDateString(new Date()));

const buckets = ref<any[]>([]);
const spendingCats = ref<any[]>([]);
const monthCats = ref<any[]>([]);
const selectedKey = ref<string | null>(null);
const drillTxns = ref<any[]>([]);
const drillTotal = ref(0);
const drillPage = ref(0);
const drillSort = ref<'debit-desc' | 'debit-asc' | 'credit-desc' | 'credit-asc'>('debit-desc');
const DRILL_PAGE_SIZE = 10;

const SORTS = [
  { key: 'debit-desc' as const, label: 'Debit ↓' },
  { key: 'debit-asc' as const, label: 'Debit ↑' },
  { key: 'credit-desc' as const, label: 'Credit ↓' },
  { key: 'credit-asc' as const, label: 'Credit ↑' }
];

const RANGE_TABS = [
  { key: '7D', label: '7D' },
  { key: '1M', label: '1M' },
  { key: '6M', label: '6M' },
  { key: '1Y', label: '1Y' },
  { key: 'CUSTOM', label: 'Custom' }
];

const accountOptions = computed(() => [
  { value: '', label: 'All accounts' },
  ...accountList.value.map((a: any) => ({ value: a.id, label: a.nickname || a.bankName }))
]);

const signed = (t: any) => (t.type === 'DEBIT' ? -Math.abs(t.amount || 0) : Math.abs(t.amount || 0));
const pageSum = computed(() => drillTxns.value.reduce((s: number, t: any) => s + signed(t), 0));
const totalPages = computed(() => Math.max(1, Math.ceil(drillTotal.value / DRILL_PAGE_SIZE)));
/** Period spending: the sum of the buckets, or of the one bucket you selected. */
const periodTotal = computed(() =>
  selectedKey.value
    ? chartBuckets.value.find((b) => b.key === selectedKey.value)?.amount ?? 0
    : chartBuckets.value.reduce((sum, b) => sum + b.amount, 0)
);
const selectedLabel = computed(() => chartBuckets.value.find((b) => b.key === selectedKey.value)?.label ?? '');
/** API keys are `2026-11-10` or `2026-11`: the axis wants `10 Nov` / `Nov 26`. */
function bucketLabel(key: string): string {
  if (/^\d{4}-\d{2}-\d{2}$/.test(key)) {
    return new Date(`${key}T00:00:00`).toLocaleDateString(undefined, { day: 'numeric', month: 'short' });
  }
  if (/^\d{4}-\d{2}$/.test(key)) {
    return new Date(`${key}-01T00:00:00`).toLocaleDateString(undefined, { month: 'short', year: '2-digit' });
  }
  return key;
}

const chartBuckets = computed(() =>
  buckets.value.map((b: any) => ({ key: b.key, label: bucketLabel(b.label || b.key), amount: b.amount || 0 }))
);

/** `8–14 Nov 2026` for the header, from the active range. */
const rangeText = computed(() => {
  const r = fullRange();
  const from = new Date(`${r.from}T00:00:00`);
  const to = new Date(`${r.to}T00:00:00`);
  const d = (x: Date) => x.toLocaleDateString(undefined, { day: 'numeric' });
  const m = (x: Date) => x.toLocaleDateString(undefined, { month: 'short' });
  const y = (x: Date) => x.getFullYear();
  if (from.getMonth() === to.getMonth() && y(from) === y(to)) return `${d(from)}–${d(to)} ${m(to)} ${y(to)}`;
  return `${d(from)} ${m(from)} – ${d(to)} ${m(to)} ${y(to)}`;
});
const categoryBars = computed(() =>
  [...spendingCats.value]
    .map((c: any) => ({ key: c.id, label: c.name, amount: Math.abs(c.debit || 0) }))
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
    .map((c: any) => ({ id: c.id, name: c.name, spent: Math.abs(c.debit || 0) }))
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

function categoryName(t: any): string {
  const ids: string[] = t.categoryIds || [];
  if (!ids.length) return '—';
  const names = ids
    .map((id) => categoryList.value.find((c: any) => c.id === id)?.name)
    .filter(Boolean) as string[];
  return names.length ? names.join(', ') : '—';
}

const shortDate = (iso: string) => (iso || '').slice(0, 10);

function rangeParams(): { from?: string; to?: string } {
  if (range.value === 'CUSTOM') return { from: customStart.value, to: customEnd.value };
  return {};
}

function everyKey(from: string, to: string, word: 'day' | 'month'): string[] {
  const keys: string[] = [];
  const day = new Date(`${from}T00:00:00`);
  const end = new Date(`${to}T00:00:00`);
  if (word === 'day') {
    for (let d = day; d <= end; d = new Date(d.getTime() + 86400000)) keys.push(toLocalDateString(d));
  } else {
    for (let d = new Date(day.getFullYear(), day.getMonth(), 1); d <= end; d = new Date(d.getFullYear(), d.getMonth() + 1, 1)) {
      keys.push(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`);
    }
  }
  return keys;
}

/** Days with no spending are zero, not absent: the line must not skip them. */
function fillGaps(raw: any[]): any[] {
  const word = bucketWord();
  // Always the whole period: selecting a bucket narrows the ledger, not the chart.
  const r = fullRange();
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

// The calendar month behind the budget card, independent of the range tabs.
async function loadMonth() {
  const now = new Date();
  const from = new Date(now.getFullYear(), now.getMonth(), 1);
  const to = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  const resp = await analytics().spending({
    dateFrom: toLocalDateString(from),
    dateTo: toLocalDateString(to),
    accountId: store.state.selectedAccountId || undefined
  });
  monthCats.value = resp.categories || [];
}

function fullRange(): { from: string; to: string } {
  const now = new Date();
  if (range.value === '7D') return { from: toLocalDateString(new Date(now.getTime() - 6 * 86400000)), to: toLocalDateString(now) };
  if (range.value === '1M') return { from: toLocalDateString(new Date(now.getFullYear(), now.getMonth() - 1, now.getDate())), to: toLocalDateString(now) };
  if (range.value === '6M') return { from: toLocalDateString(new Date(now.getFullYear(), now.getMonth() - 6, now.getDate())), to: toLocalDateString(now) };
  if (range.value === '1Y') return { from: toLocalDateString(new Date(now.getFullYear() - 1, now.getMonth(), now.getDate())), to: toLocalDateString(now) };
  return { from: customStart.value, to: customEnd.value };
}

function selectedRange(): { from: string; to: string } | null {
  if (!selectedKey.value) return null;
  if (bucketWord() === 'day') return { from: selectedKey.value, to: selectedKey.value };
  const [y, m] = selectedKey.value.split('-').map(Number);
  const last = new Date(y, m, 0).getDate();
  return { from: `${selectedKey.value}-01`, to: `${selectedKey.value}-${String(last).padStart(2, '0')}` };
}

function bucketWord(): 'day' | 'month' {
  if (range.value === 'CUSTOM') {
    const days = Math.round((new Date(customEnd.value).getTime() - new Date(customStart.value).getTime()) / 86400000);
    return days <= 30 ? 'day' : 'month';
  }
  return range.value === '7D' || range.value === '1M' ? 'day' : 'month';
}

async function loadDrillPage(page: number) {
  const r = selectedRange() || fullRange();
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

async function goPage(p: number) {
  if (p < 0 || p >= totalPages.value) return;
  drillPage.value = p;
  await loadDrillPage(p);
}


watch(range, async () => {
  selectedKey.value = null;
  await refresh();
});
watch(customStart, async (v) => {
  if (v && customEnd.value && v > customEnd.value) customEnd.value = v;
  if (range.value === 'CUSTOM') {
    selectedKey.value = null;
    await refresh();
  }
});
watch(customEnd, async (v) => {
  if (v && customStart.value && v < customStart.value) customStart.value = v;
  if (range.value === 'CUSTOM') {
    selectedKey.value = null;
    await refresh();
  }
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
    const [accResp, catResp] = await Promise.all([accounts().listAccounts({}), categories().listCategories({ pageSize: 10000 })]);
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
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col gap-3">
    <!-- title, the period that governs everything, and the action -->
    <header class="flex shrink-0 flex-wrap items-center justify-between gap-3">
      <h1 class="text-[22px] leading-none text-ink">Spending</h1>
      <div class="flex flex-wrap items-center gap-3">
        <AppTabs v-model="range" :items="RANGE_TABS" />
        <span class="tabular whitespace-nowrap text-[12px] text-ink-soft">{{ rangeText }}</span>
        <AppButton @click="adding = true">Add entry</AppButton>
      </div>
    </header>

    <div v-if="range === 'CUSTOM'" class="flex shrink-0 items-end gap-2">
      <AppField v-model="customStart" type="date" label="From" class="w-[160px]" />
      <AppField v-model="customEnd" type="date" label="To" class="w-[160px]" />
    </div>

    <p v-if="failed" class="flex shrink-0 items-center justify-between rounded-card border border-debit/40 px-3.5 py-2.5 text-[12.5px] text-debit">
      Spending could not be loaded.
      <AppButton variant="quiet" size="sm" @click="refresh">Retry</AppButton>
    </p>

    <AppStamp v-if="selectedKey" tone="accent" class="shrink-0 cursor-pointer self-start" @click="selectedKey = null; refresh()">
      {{ selectedLabel }} ✕
    </AppStamp>

    <div class="grid min-h-0 flex-1 grid-cols-1 gap-3 lg:grid-cols-12 lg:grid-rows-[auto_minmax(0,0.9fr)_minmax(0,1.1fr)]">
      <!-- the four facts of the period, each with a second line that means something -->
      <div class="grid grid-cols-2 gap-3 lg:col-span-12 lg:grid-cols-[1.5fr_1fr_1fr_1fr]">
        <AppFolio class="justify-center" body-class="flex flex-col justify-center">
          <p class="text-label font-medium uppercase tracking-[0.07em] text-ink-mute">Budget · {{ monthLabel }}</p>
          <template v-if="hasBudgets">
            <p class="tabular mt-1 flex items-baseline gap-1.5 font-display text-[26px] leading-none tracking-[-0.02em]" :class="budgetTotals.over ? 'text-debit' : 'text-ink'">
              {{ budgetTotals.over ? '−' : '' }}{{ formatCurrency(Math.abs(budgetTotals.left)) }}
              <span class="font-ui text-[11px] tracking-normal text-ink-mute">{{ budgetTotals.over ? 'over' : 'left' }}</span>
            </p>
            <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-page-sunk">
              <div
                class="h-full rounded-full"
                :class="budgetTotals.over ? 'bg-debit' : 'bg-accent'"
                :style="{ width: `${Math.max(budgetTotals.used, 2)}%` }"
              ></div>
            </div>
            <p class="mt-1.5 text-[11.5px] text-ink-mute">
              {{ formatCurrency(budgetTotals.spent) }} of {{ formatCurrency(budgetTotals.planned) }} · {{ Math.round(budgetTotals.used) }}% used
            </p>
          </template>
          <template v-else>
            <p class="mt-1 text-[20px] leading-none text-ink">No budgets yet</p>
            <p class="mt-1.5 text-[11.5px] text-ink-mute">
              <router-link to="/accounts/categories" class="text-accent hover:text-accent-hover">Set monthly limits →</router-link>
            </p>
          </template>
        </AppFolio>
        <AppFolio class="justify-center" body-class="flex flex-col justify-center">
          <p class="text-label font-medium uppercase tracking-[0.07em] text-ink-mute">Spent</p>
          <p v-if="loading" class="mt-1.5 h-7 w-28 animate-pulse rounded-full bg-page-sunk"></p>
          <p v-else class="tabular mt-1 font-display text-[26px] leading-none tracking-[-0.02em] text-ink">{{ formatCurrency(periodTotal) }}</p>
          <p class="mt-1.5 text-[11.5px] text-ink-mute">{{ drillTotal }} entries in this period</p>
        </AppFolio>
        <AppFolio class="justify-center" body-class="flex flex-col justify-center">
          <p class="text-label font-medium uppercase tracking-[0.07em] text-ink-mute">Categories</p>
          <p class="tabular mt-1 text-[20px] leading-none text-ink">{{ composition.length }}</p>
          <p class="mt-1.5 text-[11.5px] text-ink-mute">{{ chartBuckets.length }} {{ bucketWord() === 'day' ? 'days' : 'months' }} in range</p>
        </AppFolio>
        <AppFolio class="justify-center" body-class="flex flex-col justify-center">
          <p class="text-label font-medium uppercase tracking-[0.07em] text-ink-mute">Average / day</p>
          <p class="tabular mt-1 text-[20px] leading-none text-ink">{{ formatCurrency(periodTotal / Math.max(1, chartBuckets.length)) }}</p>
          <p class="mt-1.5 text-[11.5px] text-ink-mute">across the selected range</p>
        </AppFolio>
      </div>

      <!-- when the money went -->
      <AppFolio
        title="Spending over time"
        subtitle="click a day to narrow the ledger"
        class="min-h-0 lg:col-span-7"
        body-class="flex min-h-0 flex-1 flex-col justify-center"
      >
        <SpendingColumnsChart
          v-if="!loading && chartBuckets.length"
          :points="chartBuckets"
          :selected-key="selectedKey"
          :height="196"
          @select="selectBucket"
        />
        <span v-else-if="loading" class="block h-[196px] animate-pulse rounded-control bg-page-sunk" />
        <AppEmpty v-else title="Nothing in this range." hint="Try a wider period, or clear the account filter." />
      </AppFolio>

      <!-- what it went on: monthly budgets take precedence, otherwise the range share -->
      <AppFolio
        :title="hasBudgets ? 'Budget by category' : 'Where it went'"
        :subtitle="hasBudgets ? `this month vs monthly budget · ${monthLabel}` : 'top 6 by spend · others folded in'"
        class="min-h-0 lg:col-span-5"
        body-class="flex min-h-0 flex-1 flex-col"
        flush
      >
        <div v-if="hasBudgets" class="flex min-h-0 flex-1 flex-col overflow-auto">
          <div
            v-for="c in budgetedCats"
            :key="c.id"
            class="border-b border-rule-soft px-4 py-2.5 last:border-0"
            :class="c.over ? 'bg-debit/5' : ''"
          >
            <div class="flex items-baseline gap-2">
              <span class="h-[9px] w-[9px] shrink-0 rounded-full" :class="c.over ? 'bg-debit' : 'bg-accent'" aria-hidden="true" />
              <span class="min-w-0 flex-1 truncate text-[12.5px] text-ink-soft">{{ c.name }}</span>
              <span class="tabular whitespace-nowrap text-[12px] text-ink-mute">{{ formatCurrency(c.spent) }} / {{ formatCurrency(c.budget) }}</span>
              <span class="tabular w-[86px] shrink-0 text-right text-[12px] font-medium" :class="c.over ? 'text-debit' : 'text-credit'">
                {{ c.over ? '−' : '' }}{{ formatCurrency(Math.abs(c.left)) }} {{ c.over ? 'over' : 'left' }}
              </span>
            </div>
            <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-page-sunk">
              <div class="h-full rounded-full" :class="c.over ? 'bg-debit' : 'bg-accent'" :style="{ width: `${Math.max(c.used, 1.5)}%` }"></div>
            </div>
          </div>
          <div
            v-for="c in unbudgetedCats"
            :key="c.id"
            class="flex items-center gap-2.5 border-b border-rule-soft px-4 py-2 last:border-0"
          >
            <span class="h-[9px] w-[9px] shrink-0 rounded-full bg-ink-faint" aria-hidden="true" />
            <span class="min-w-0 flex-1 truncate text-[12.5px] text-ink-soft">{{ c.name }}</span>
            <span class="tabular text-[12px] text-ink">{{ formatCurrency(c.spent) }}</span>
            <span class="w-[86px] shrink-0 text-right text-[11.5px] text-ink-faint">unbudgeted</span>
          </div>
        </div>

        <div v-else-if="composition.length" class="flex min-h-0 flex-1 flex-col">
          <div class="px-4 pt-4">
            <CategoryShareBar :items="composition" :height="30" />
          </div>
          <div class="mt-6 flex min-h-0 flex-1 flex-col gap-0 overflow-auto px-4 pb-2">
            <div v-for="(c, i) in composition" :key="c.name" class="flex items-center gap-2.5 border-b border-rule-soft py-[7px] last:border-0">
              <span class="h-[9px] w-[9px] shrink-0 rounded-full" :style="{ background: `var(--chart-${Math.min(i, 7)})` }" />
              <span class="min-w-0 flex-1 truncate text-[12.5px] text-ink-soft">{{ c.name }}</span>
              <span class="tabular text-[12.5px] font-medium text-ink">{{ formatCurrency(c.amount) }}</span>
            </div>
          </div>
        </div>
        <span v-else-if="loading" class="m-4 block h-[140px] animate-pulse rounded-control bg-page-sunk" />
        <div v-else class="px-4 py-6"><AppEmpty title="No categorised spending." hint="Categories appear here once entries carry one." /></div>
      </AppFolio>

      <!-- the entries themselves -->
      <AppFolio
        title="Ledger"
        flush
        class="flex min-h-0 flex-col lg:col-span-12"
        body-class="flex min-h-0 flex-1 flex-col"
      >
        <template #actions>
          <span class="text-[12px] text-ink-mute">{{ drillTotal }} {{ drillTotal === 1 ? 'entry' : 'entries' }}</span>
        </template>

        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-rule px-4 py-2">
          <AppSelect
            :model-value="store.state.selectedAccountId || ''"
            :options="accountOptions"
            class="w-[172px]"
            @update:model-value="(v: string) => store.select(v || null)"
          />
          <div class="flex items-center gap-1.5">
            <button
              v-for="opt in SORTS"
              :key="opt.key"
              type="button"
              class="h-[30px] rounded-control border px-2.5 text-[12px] font-medium transition-colors"
              :class="drillSort === opt.key ? 'border-accent/40 bg-accent-wash text-accent' : 'border-transparent text-ink-soft hover:bg-page-sunk hover:text-ink'"
              @click="drillSort = opt.key; refresh()"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>

        <div class="flex min-h-0 flex-1 flex-col overflow-auto">
          <AppTable
            :margin="false"
            :columns="[
              { label: 'Entry' },
              { label: 'Category', hideOnMobile: true },
              { label: 'Amount', align: 'right', width: '140px' }
            ]"
            :loading="loading"
            :empty="!drillTxns.length"
          >
            <tr v-for="t in drillTxns" :key="t.id">
              <td class="border-b border-rule-soft px-3 py-2.5">
                <span class="block truncate text-[13px] text-ink">{{ t.name }}</span>
                <span class="block text-[11px] text-ink-mute">{{ shortDate(t.occurredAt) }}</span>
              </td>
              <td class="hidden border-b border-rule-soft px-3 py-2.5 text-[12px] text-ink-soft md:table-cell">{{ categoryName(t) }}</td>
              <td class="tabular border-b border-rule-soft px-3 py-2.5 text-right text-[13px] font-medium" :class="t.type === 'DEBIT' ? 'text-debit' : 'text-credit'">
                {{ t.type === 'DEBIT' ? '−' : '+' }}{{ formatCurrency(Math.abs(t.amount || 0)) }}
              </td>
            </tr>
            <template #empty>
              <AppEmpty title="No entries in this range." hint="Widen the period, clear the account filter, or add an entry.">
                <template #action><AppButton size="sm" @click="adding = true">Add entry</AppButton></template>
              </AppEmpty>
            </template>
          </AppTable>

          <!-- The carried-forward line closes the page: it sits at the bottom of
               the ledger even when the range has only a few rows. -->
          <div v-if="!loading && drillTxns.length" class="mt-auto flex items-center border-t border-rule bg-page-sunk px-3 py-2.5">
            <span class="flex-1 text-[12px] text-ink-mute">Carried forward · this page</span>
            <span class="tabular w-[140px] pr-3 text-right font-display text-[15px] text-ink">{{ formatCurrency(pageSum) }}</span>
          </div>
        </div>

        <template #footer>
          <div class="flex items-center justify-between">
            <span class="text-[12px] text-ink-mute">{{ drillTotal }} entries in the selected range</span>
            <div class="flex items-center gap-1">
              <AppButton variant="quiet" size="sm" :disabled="drillPage <= 0" @click="goPage(drillPage - 1)">‹</AppButton>
              <span class="tabular px-1 text-[12px] text-ink-soft">{{ drillPage + 1 }} / {{ totalPages }}</span>
              <AppButton variant="quiet" size="sm" :disabled="drillPage + 1 >= totalPages" @click="goPage(drillPage + 1)">›</AppButton>
            </div>
          </div>
        </template>
      </AppFolio>
    </div>

    <AddTransactionModal
      v-if="adding"
      :accounts="accountList"
      :categories="categoryList"
      :default-account-id="store.state.selectedAccountId || accountList[0]?.id || ''"
      @close="adding = false"
      @imported="adding = false; refresh()"
    />
  </div>
</template>
