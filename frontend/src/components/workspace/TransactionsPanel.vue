<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue';
import TransactionsTab from '@/components/workspace/TransactionsTab.vue';
import AddTransactionModal from '@/components/workspace/AddTransactionModal.vue';
import EditItemModal from '@/components/workspace/EditItemModal.vue';
import TransferModal from '@/components/workspace/TransferModal.vue';
import TransactionFilterInput from '@/components/workspace/TransactionFilterInput.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import Pagination from '@/components/Pagination.vue';
import StatementImportModal from '@/components/workspace/StatementImportModal.vue';
import { accounts, transactions, categories, transfers } from '@/lib/api/client';
import { emptyFilters, type TransactionFilters } from '@/lib/utils/transactionFilters';
import { useAccountsStore } from '@/lib/stores/accounts';
import { Plus, Upload } from '@lucide/vue';
import AppFolio from '@/components/ui/AppFolio.vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppEmpty from '@/components/ui/AppEmpty.vue';

const store = useAccountsStore();

const emit = defineEmits<{ (e: 'updated'): void }>();

const accountList = ref<any[]>([]);
const txnList = ref<any[]>([]);
const categoryList = ref<any[]>([]);
const loading = ref(true);
const error = ref('');

const filters = ref<TransactionFilters>(emptyFilters());
const page = ref(0);
const DEFAULT_PAGE_SIZE = 20;
const pageSize = ref(DEFAULT_PAGE_SIZE);
// Cursor pagination: tokens[n] is the token that fetches page n.
const tokens = ref<string[]>(["" ]);
const reachable = ref(0);
const listEl = ref<HTMLElement | null>(null);
const totalCount = ref(0);

const showImport = ref(false);
const showAddModal = ref(false);
const showItemModal = ref(false);
const editingItem = ref<any>(null);
const showTransfer = ref(false);
const transferPrefill = ref<{ sourceId: string; fromId: string } | null>(null);
const confirmDelete = ref<{ ids: string[]; label: string } | null>(null);

const pagedTxns = computed(() => txnList.value);
const totalPages = computed(() => Math.max(1, Math.ceil(totalCount.value / pageSize.value)));

async function loadPage() {
  loading.value = true;
  error.value = '';
  const cursor = tokens.value[page.value] ?? '';
  try {
    const resp = await transactions().listTransactions({
      pageSize: pageSize.value,
      accountId: store.state.selectedAccountId || undefined,
      dateFrom: filters.value.dateFrom || undefined,
      dateTo: filters.value.dateTo || undefined,
      minAmount: filters.value.minAmount || undefined,
      maxAmount: filters.value.maxAmount || undefined,
      categoryId: filters.value.categoryId || undefined,
      names: filters.value.names,
      type: filters.value.type || undefined,
      ...(cursor ? { pageToken: cursor } : {})
    });
    txnList.value = (resp.transactions || []);
    totalCount.value = resp.totalCount || txnList.value.length;
    if (resp.nextPageToken) {
      tokens.value = [...tokens.value.slice(0, page.value + 1), resp.nextPageToken];
      reachable.value = page.value + 1;
    } else {
      tokens.value = tokens.value.slice(0, page.value + 1);
      reachable.value = page.value;
    }
  } catch (e: any) {
    error.value = e?.message || 'Failed to load transactions';
    txnList.value = [];
    totalCount.value = 0;
  }
  loading.value = false;
}

function resetPaging() {
  page.value = 0;
  tokens.value = [''];
  reachable.value = 0;
}

function changePage(next: number) {
  if (next < 0 || next > reachable.value) return;
  page.value = next;
  loadPage();
}

/** Rows that fit the visible ledger area: the table scrolls, the pager does not. */
function fitPageSize() {
  const el = listEl.value;
  if (!el) return;
  const height = el.clientHeight;
  if (!height) return;
  const row = el.querySelector('tbody tr');
  const rowHeight = row ? row.getBoundingClientRect().height : 44;
  const header = el.querySelector('thead')?.getBoundingClientRect().height ?? 0;
  const rows = Math.max(5, Math.min(200, Math.floor((height - header - 2) / (rowHeight || 44))));
  if (rows === pageSize.value) return;
  pageSize.value = rows;
  resetPaging();
  loadPage();
}

let resizeObserver: ResizeObserver | null = null;
onMounted(() => {
  resizeObserver = new ResizeObserver(fitPageSize);
  if (listEl.value) resizeObserver.observe(listEl.value);
});
watch(listEl, (el, old) => {
  if (old) resizeObserver?.unobserve(old);
  if (el) {
    resizeObserver?.observe(el);
    nextTick(fitPageSize);
  }
});
onBeforeUnmount(() => resizeObserver?.disconnect());

async function loadMeta() {
  try {
    const [accResp, catResp] = await Promise.all([
      accounts().listAccounts({}),
      categories().listCategories({ pageSize: 10000 })
    ]);
    accountList.value = accResp.accounts || [];
    categoryList.value = catResp.categories || [];
  } catch (e) { console.error(e); }
}

watch(filters, () => { resetPaging(); loadPage(); }, { deep: true });
watch(() => store.state.selectedAccountId, () => { resetPaging(); loadPage(); });

onMounted(() => { loadMeta(); loadPage(); nextTick(fitPageSize); });

function openAdd() {
  showAddModal.value = true;
}
function openEditItem(item: any) {
  editingItem.value = item; showItemModal.value = true;
}
function openTransfer(txn: any) {
  transferPrefill.value = { sourceId: txn.id, fromId: txn.accountId };
  showTransfer.value = true;
}
async function unlinkTransfer() {
  const linkId = editingItem.value?.linkedTransferId;
  if (!linkId) return;
  try {
    await transfers().unlinkTransfers({ ids: [linkId] });
    showItemModal.value = false;
    await loadPage();
    emit('updated');
  } catch (e) { console.error(e); }
}
async function submitItem(payload: any) {
  try {
    await transactions().updateTransactions({ transactions: [{ ...payload, id: editingItem.value.id }] });
    showItemModal.value = false;
    await loadPage();
    emit('updated');
  } catch (e) { console.error(e); }
}
function requestDeleteItem(item: any) {
  confirmDelete.value = { ids: [item.id], label: item.name };
}
function requestBulkDelete(ids: string[]) {
  confirmDelete.value = { ids, label: `${ids.length} transactions` };
}
async function confirmDeleteNow() {
  if (!confirmDelete.value) return;
  try {
    await transactions().deleteTransactions({ ids: confirmDelete.value.ids });
    showItemModal.value = false;
    await loadPage();
    emit('updated');
  } catch (e) { console.error(e); }
  confirmDelete.value = null;
}
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col gap-3">
    <p v-if="error" class="flex shrink-0 items-center justify-between rounded-card border border-debit/40 px-3.5 py-2.5 text-[12.5px] text-debit">
      <span>{{ error }}</span>
      <AppButton variant="quiet" size="sm" @click="loadPage">Retry</AppButton>
    </p>

    <AppFolio v-if="!loading" flush class="min-h-0 flex-1" body-class="flex min-h-0 flex-1 flex-col overflow-hidden">
      <template #default>
        <div class="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-rule px-4 py-2">
          <div class="flex min-w-0 flex-1 items-center gap-2">
            <h2 class="shrink-0 text-[15px] font-semibold text-ink">Transactions</h2>
            <TransactionFilterInput v-model:filters="filters" :categories="categoryList" />
          </div>
          <div class="flex items-center gap-2">
            <AppButton variant="outline" @click="showImport = true">
              <Upload class="h-[15px] w-[15px]" stroke-width="1.8" />
              Import
            </AppButton>
            <AppButton @click="openAdd">
              <Plus class="h-[15px] w-[15px]" stroke-width="1.8" />
              Add
            </AppButton>
          </div>
        </div>

        <div ref="listEl" class="min-h-0 flex-1 overflow-auto">
          <div v-if="txnList.length === 0" class="px-4 py-10">
            <AppEmpty title="No transactions yet." hint="Record an expense or income to start the books.">
              <template #action><AppButton size="sm" @click="openAdd">Add transaction</AppButton></template>
            </AppEmpty>
          </div>

          <TransactionsTab
            v-else
            v-model:filters="filters"
            :paged-txns="pagedTxns"
            :filtered-count="totalCount"
            :total-pages="totalPages"
            :categories="categoryList"
            :all-txns="txnList"
            :accounts="accountList"
            @edit="openEditItem"
            @bulk-delete="requestBulkDelete"
            @transfer="openTransfer"
          />
        </div>

        <Pagination
          v-if="totalPages > 1"
          :page="page"
          :total-pages="totalPages"
          :total-count="totalCount"
          :page-size="pageSize"
          :reachable="reachable"
          @page-change="changePage"
        />
      </template>
    </AppFolio>

    <div v-else class="rounded-card border border-rule bg-page p-4">
      <div class="flex flex-col gap-2">
        <span v-for="i in 5" :key="i" class="block h-9 animate-pulse rounded-control bg-page-sunk" />
      </div>
    </div>

    <StatementImportModal
      v-if="showImport"
      :accounts="accountList"
      :default-account-id="store.state.selectedAccountId || accountList[0]?.id || ''"
      @close="showImport = false"
      @imported="loadPage(); emit('updated')"
    />

    <AddTransactionModal
      v-if="showAddModal"
      :accounts="accountList"
      :categories="categoryList"
      :default-account-id="store.state.selectedAccountId || accountList[0]?.id || ''"
      @close="showAddModal = false"
      @imported="loadMeta(); loadPage(); emit('updated')"
    />

    <EditItemModal
      v-if="showItemModal"
      :key="editingItem?.id || 'new'"
      type="transaction"
      :editing-item="editingItem"
      :accounts="accountList"
      :categories="categoryList"
      :default-account-id="store.state.selectedAccountId || accountList[0]?.id || ''"
      @close="showItemModal = false"
      @submit="submitItem"
      @delete="requestDeleteItem(editingItem)"
      @unlink="unlinkTransfer"
    />

    <TransferModal v-if="showTransfer" :txns="txnList" :accounts="accountList" :prefill="transferPrefill || undefined" @close="showTransfer = false" @done="loadPage(); emit('updated')" />

    <ConfirmDialog
      v-if="confirmDelete"
      title="Delete transaction"
      :message="`Delete &quot;${confirmDelete.label}&quot;? This can't be undone.`"
      @confirm="confirmDeleteNow"
      @cancel="confirmDelete = null"
    />
  </div>
</template>