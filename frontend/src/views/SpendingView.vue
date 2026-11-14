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
const SpendingTrendChart = defineAsyncComponent(() => import('@/components/ui/SpendingTrendChart.vue'));
const CategoryShareBar = defineAsyncComponent(() => import('@/components/ui/CategoryShareBar.vue'));
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
    await Promise.all([loadSpending(), loadDrillPage(0)]);
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
    <!-- header: the figures, and the period that produces them -->
    <header class="shrink-0">
      <div class="flex items-center justify-between gap-4">
        <h1 class="text-[22px] leading-none text-ink">Spending</h1>
        <AppButton @click="adding = true">Add entry</AppButton>
      </div>
      <div class="mt-2 flex flex-wrap items-end justify-between gap-x-6 gap-y-2 border-b border-rule pb-2.5">
        <div class="flex items-stretch">
          <div class="flex flex-col justify-end gap-1 pr-5">
            <span class="text-label font-medium uppercase tracking-[0.07em] text-ink-mute">Spent</span>
            <span class="tabular font-display text-[28px] leading-none tracking-[-0.02em] text-ink">{{ formatCurrency(periodTotal) }}</span>
          </div>
          <div class="flex flex-col justify-end gap-1 border-l border-rule px-5">
            <span class="text-label font-medium uppercase tracking-[0.07em] text-ink-mute">Entries</span>
            <span class="tabular text-[15px] leading-none text-ink">{{ drillTotal }}</span>
          </div>
          <div class="flex flex-col justify-end gap-1 border-l border-rule pl-5">
            <span class="text-label font-medium uppercase tracking-[0.07em] text-ink-mute">Largest</span>
            <span class="text-[15px] leading-none text-ink">
              {{ composition[0]?.name || '—' }}<template v-if="composition.length"> · {{ formatCurrency(composition[0].amount) }}</template>
            </span>
          </div>
        </div>
        <div class="flex items-end gap-3">
          <AppTabs v-model="range" :items="RANGE_TABS" />
          <span class="tabular pb-[9px] text-[12px] text-ink-soft">{{ rangeText }}</span>
          <AppStamp v-if="selectedKey" tone="accent" class="mb-[6px] cursor-pointer" @click="selectedKey = null; refresh()">
            {{ selectedLabel }} ✕
          </AppStamp>
        </div>
      </div>
      <div v-if="range === 'CUSTOM'" class="mt-2 flex items-end gap-2">
        <AppField v-model="customStart" type="date" label="From" class="w-[160px]" />
        <AppField v-model="customEnd" type="date" label="To" class="w-[160px]" />
        <span class="pb-2 text-[12px] text-ink-mute">custom range feeds the chart, the breakdown and the ledger</span>
      </div>
    </header>

    <p v-if="failed" class="flex items-center justify-between rounded-card border border-debit/40 px-4 py-2.5 text-[13px] text-debit">
      Spending could not be loaded.
      <AppButton variant="quiet" size="sm" @click="refresh">Retry</AppButton>
    </p>

    <div class="grid min-h-0 flex-1 grid-cols-1 gap-3 lg:grid-cols-2">
      <div class="grid min-h-0 grid-rows-2 gap-3">
        <!-- spending over time: smooth line, click a point to narrow -->
        <AppFolio title="Spending over time" subtitle="click a point to narrow the ledger" class="min-h-0" body-class="flex min-h-0 flex-1 flex-col px-0 py-0">
          <div class="relative grid min-h-0 flex-1 place-items-center">
            <SpendingTrendChart
              v-if="!loading && chartBuckets.length"
              :points="chartBuckets"
              :selected-key="selectedKey"
              fill
              @select="selectBucket"
            />
            <span v-else-if="loading" class="absolute inset-0 animate-pulse rounded-control bg-page-sunk" />
            <div v-else class="flex h-full items-center justify-center">
              <AppEmpty title="Nothing in this range." hint="Try a wider period, or clear the account filter." />
            </div>
          </div>
        </AppFolio>

        <!-- composition: one 100% bar, amounts in the legend -->
        <AppFolio title="Where it went" subtitle="top 6 by spend · others folded in" class="min-h-0" body-class="flex min-h-0 flex-1 flex-col justify-center" flush>
          <div v-if="composition.length" class="flex flex-col justify-center px-4 py-3">
            <CategoryShareBar :items="composition" :height="40" />
            <div class="mt-2.5 flex flex-col">
              <div v-for="(c, i) in composition" :key="c.name" class="flex items-center gap-2.5 border-b border-rule-soft py-[6px] last:border-0">
                <span class="h-[9px] w-[9px] shrink-0 rounded-full" :style="{ background: `var(--chart-${i})` }" />
                <span class="min-w-0 flex-1 truncate text-[12.5px] text-ink-soft">{{ c.name }}</span>
                <span class="tabular text-[12.5px] font-medium text-ink">{{ formatCurrency(c.amount) }}</span>
              </div>
            </div>
          </div>
          <span v-else-if="loading" class="m-4 block h-[150px] animate-pulse rounded-control bg-page-sunk" />
          <AppEmpty v-else title="No categorised spending." hint="Categories appear here once entries carry one." />
        </AppFolio>
      </div>

      <AppFolio title="Ledger" flush class="flex min-h-0 flex-1 flex-col" body-class="flex min-h-0 flex-1 flex-col">
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
          <template #footer>
            <tr>
              <td class="bg-page-sunk px-3 py-2.5 text-[12px] text-ink-mute">Carried forward · this page</td>
              <td class="tabular bg-page-sunk px-3 py-2.5 text-right font-display text-[15px] text-ink">{{ formatCurrency(pageSum) }}</td>
            </tr>
          </template>
        </AppTable>
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
