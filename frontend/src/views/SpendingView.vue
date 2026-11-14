<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue';
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
import AppTick from '@/components/ui/AppTick.vue';
import AppLineChart from '@/components/ui/AppLineChart.vue';
import AppBars from '@/components/ui/AppBars.vue';
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
const chartBuckets = computed(() =>
  buckets.value.map((b: any) => ({ key: b.key, label: b.label || b.key, amount: b.amount || 0 }))
);
const categoryBars = computed(() =>
  [...spendingCats.value]
    .map((c: any) => ({ key: c.id, label: c.name, amount: Math.abs(c.debit || 0) }))
    .filter((c) => c.amount > 0)
    .sort((a, b) => b.amount - a.amount)
);

function accountName(id: string): string {
  const acc = accountList.value.find((a: any) => a.id === id);
  return acc ? acc.nickname || acc.bankName : '—';
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

function cycleSort() {
  drillSort.value = drillSort.value === 'debit-desc' ? 'debit-asc' : 'debit-desc';
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
  <div class="flex flex-col gap-5">
    <header class="flex items-end justify-between gap-4">
      <div>
        <h1 class="text-[26px] leading-tight text-ink">Spending</h1>
        <p class="mt-1 text-[13px] text-ink-mute">The ledger, by period and category.</p>
      </div>
      <AppButton @click="adding = true">Add entry</AppButton>
    </header>

    <!-- filters: a tab row, an account, and the dates only when they matter -->
    <div class="flex flex-col gap-3 border-b border-rule pb-2">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <AppTabs v-model="range" :items="RANGE_TABS" class="min-w-[280px] flex-1" />
        <div class="flex items-center gap-2">
          <AppSelect
            :model-value="store.state.selectedAccountId || ''"
            :options="accountOptions"
            class="w-[190px]"
            @update:model-value="(v: string) => store.select(v || null)"
          />
          <AppButton variant="quiet" size="sm" :label="drillSort === 'debit-desc' ? 'Amount, high to low' : 'Amount, low to high'" @click="cycleSort">
            <span class="text-[12px]">Amount</span>
            <span class="tabular text-[12px]">{{ drillSort === 'debit-desc' ? '↓' : '↑' }}</span>
          </AppButton>
        </div>
      </div>
      <div v-if="range === 'CUSTOM'" class="flex flex-wrap items-center gap-2 pb-1">
        <AppField v-model="customStart" type="date" label="From" class="w-[170px]" />
        <AppField v-model="customEnd" type="date" label="To" class="w-[170px]" />
      </div>
    </div>

    <p v-if="failed" class="flex items-center justify-between rounded-card border border-debit/40 px-4 py-2.5 text-[13px] text-debit">
      Spending could not be loaded.
      <AppButton variant="quiet" size="sm" @click="refresh">Retry</AppButton>
    </p>

    <AppFolio title="Spending over time" :subtitle="selectedKey ? 'One bucket selected — the ledger below is narrowed to it' : 'Click a point to narrow the ledger'">
      <AppLineChart v-if="!loading && chartBuckets.length" :buckets="chartBuckets" :selected-key="selectedKey" @select="selectBucket" />
      <span v-else-if="loading" class="block h-[132px] animate-pulse rounded-control bg-page-sunk" />
      <AppEmpty v-else title="Nothing in this range." hint="Try a wider period, or clear the account filter." />
    </AppFolio>

    <AppFolio title="By category" subtitle="Debits in the selected range">
      <AppBars v-if="!loading && categoryBars.length" :bars="categoryBars" />
      <span v-else-if="loading" class="block h-[120px] animate-pulse rounded-control bg-page-sunk" />
      <AppEmpty v-else title="No categorised spending." hint="Categories appear here once entries carry one." />
    </AppFolio>

    <AppFolio title="Ledger" :subtitle="`${drillTotal} ${drillTotal === 1 ? 'entry' : 'entries'}`" flush>
      <AppTable
        :columns="[
          { label: 'Entry' },
          { label: 'Account', hideOnMobile: true },
          { label: 'Amount', align: 'right', width: '140px' }
        ]"
        :loading="loading"
        :empty="!drillTxns.length"
      >
        <tr v-for="t in drillTxns" :key="t.id">
          <td class="m-cell">
            <AppTick v-if="t.linkedTransferId" state="done" label="Matched transfer" class="mx-auto" />
          </td>
          <td class="border-b border-rule-soft px-3 py-2.5">
            <span class="block truncate text-[13px] text-ink">{{ t.name }}</span>
            <span class="block text-[11px] text-ink-mute">{{ shortDate(t.occurredAt) }}</span>
          </td>
          <td class="hidden border-b border-rule-soft px-3 py-2.5 text-[12px] text-ink-soft md:table-cell">{{ accountName(t.accountId) }}</td>
          <td
            class="tabular border-b border-rule-soft px-3 py-2.5 text-right text-[13px] font-medium"
            :class="t.type === 'DEBIT' ? 'text-debit' : 'text-credit'"
          >
            {{ t.type === 'DEBIT' ? '−' : '+' }}{{ formatCurrency(Math.abs(t.amount || 0)) }}
          </td>
        </tr>
        <template #empty>
          <AppEmpty title="No entries in this range." hint="Widen the period, clear the account filter, or add an entry.">
            <template #action>
              <AppButton size="sm" @click="adding = true">Add entry</AppButton>
            </template>
          </AppEmpty>
        </template>
        <template #footer>
          <tr>
            <td class="m-cell"></td>
            <td class="bg-page-sunk px-3 py-2.5 text-[12px] text-ink-mute" colspan="2">
              Carried forward · this page ({{ drillPage + 1 }} / {{ totalPages }})
            </td>
            <td class="tabular bg-page-sunk px-3 py-2.5 text-right font-display text-[15px] text-ink">{{ formatCurrency(pageSum) }}</td>
          </tr>
        </template>
      </AppTable>
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
