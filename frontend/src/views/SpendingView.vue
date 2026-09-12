<script setup lang="ts">
import { ref, watch, onMounted } from 'vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import AccountFilterBar from '@/components/accounts/AccountFilterBar.vue';
import ExpenseChart from '@/components/accounts/ExpenseChart.vue';
import TimeRange from '@/components/TimeRange.vue';
import DatePicker from '@/components/DatePicker.vue';
import { accounts, categories, analytics, transactions } from '@/lib/api/client';
import { useAccountsStore } from '@/lib/stores/accounts';
import { defaultSpendingRange, toLocalDateString } from '@/lib/utils/format';

const store = useAccountsStore();

const accountList = ref<any[]>([]);
const categoryList = ref<any[]>([]);
const loading = ref(true);

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

// The bar chart always shows the full range; only the drill-down table narrows
// to the selected bucket (see currentTxnRange).
function rangeParams(): { from?: string; to?: string } {
  if (range.value === 'CUSTOM') return { from: customStart.value, to: customEnd.value };
  return {};
}

async function loadSpending() {
  const q: Record<string, unknown> = { range: range.value, accountId: store.state.selectedAccountId || undefined, ...rangeParams() };
  const resp = await analytics().spending(q);
  buckets.value = resp.buckets || [];
  spendingCats.value = resp.categories || [];
}

// The transactions table shows the whole active range when no bar is selected,
// and the selected bucket's range when one is.
function currentTxnRange(): { from: string; to: string } {
  return selectedRange() || fullRange();
}

function fullRange(): { from: string; to: string } {
  const now = new Date();
  if (range.value === '7D') return { from: toLocalDateString(new Date(now.getTime() - 6 * 86400000)), to: toLocalDateString(now) };
  if (range.value === '1M') return { from: toLocalDateString(new Date(now.getFullYear(), now.getMonth() - 1, now.getDate())), to: toLocalDateString(now) };
  if (range.value === '6M') return { from: toLocalDateString(new Date(now.getFullYear(), now.getMonth() - 6, now.getDate())), to: toLocalDateString(now) };
  if (range.value === '1Y') return { from: toLocalDateString(new Date(now.getFullYear() - 1, now.getMonth(), now.getDate())), to: toLocalDateString(now) };
  return { from: customStart.value, to: customEnd.value };
}

async function loadDrillPage(page: number) {
  const r = currentTxnRange();
  const [sortBy, sortDir] = drillSort.value.split('-');
  const offset = page * DRILL_PAGE_SIZE;
  const resp = await transactions().listTransactions({
    pageSize: DRILL_PAGE_SIZE,
    offset,
    sortBy,
    sortDir,
    dateFrom: r.from,
    dateTo: r.to,
    accountId: store.state.selectedAccountId || undefined
  });
  drillTxns.value = (resp.transactions || []);
  drillTotal.value = resp.totalCount || drillTxns.value.length;
}

async function refresh() {
  drillPage.value = 0;
  await Promise.all([loadSpending(), loadDrillPage(0)]);
}

function selectedRange(): { from: string; to: string } | null {
  if (!selectedKey.value) return null;
  const gran = bucketWord();
  if (gran === 'day') {
    return { from: selectedKey.value, to: selectedKey.value };
  }
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

async function selectBucket(key: string) {
  selectedKey.value = selectedKey.value === key ? null : key;
  await refresh();
}

async function drillPageChange(p: number) {
  drillPage.value = p;
  await loadDrillPage(p);
}

watch(range, async () => { selectedKey.value = null; await refresh(); });
// End date always stays toward the future (never before the start).
watch(customStart, async (v) => {
  if (v && customEnd.value && v > customEnd.value) customEnd.value = v;
  if (range.value === 'CUSTOM') { selectedKey.value = null; await refresh(); }
});
watch(customEnd, async (v) => {
  if (v && customStart.value && v < customStart.value) customStart.value = v;
  if (range.value === 'CUSTOM') { selectedKey.value = null; await refresh(); }
});
watch(() => store.state.selectedAccountId, async () => { selectedKey.value = null; await refresh(); });
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
  } catch (e) { console.error(e); }
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
  } catch (e) { console.error(e); }
  loading.value = false;
}

onMounted(loadAll);
</script>

<template>
  <div class="w-full space-y-5">
    <SectionHeader title="Spending Tracker" subtitle="Analyze spending across your accounts" />

    <AccountFilterBar :accounts="accountList" />

    <div v-if="loading" class="text-center py-16 text-subtle">Loading...</div>
    <div v-else class="card bg-base-200 border border-border">
      <div class="card-body p-6">
        <div class="flex flex-wrap items-center justify-end gap-2 mb-4">
          <TimeRange v-model="range" />
          <template v-if="range === 'CUSTOM'">
            <div class="flex flex-wrap items-center gap-2">
              <DatePicker v-model="customStart" class="input-sm w-36" />
              <span class="text-[12px] text-subtle">to</span>
              <DatePicker v-model="customEnd" class="input-sm w-36" />
            </div>
          </template>
        </div>
        <ExpenseChart
          :buckets="buckets"
          :categories="spendingCats"
          :selected-key="selectedKey"
          :drill-txns="drillTxns"
          :drill-total="drillTotal"
          :drill-page="drillPage"
                    :drill-sort="drillSort"
          :categories-list="categoryList"
          :accounts-list="accountList"
          @select-bucket="selectBucket"
          @drill-page="drillPageChange"
          @sort-change="drillSort = $event"
        />
      </div>
    </div>
  </div>
</template>
