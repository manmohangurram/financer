<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue';
import TransactionsTab from '@/components/workspace/TransactionsTab.vue';
import AddTransactionModal from '@/components/workspace/AddTransactionModal.vue';
import EditItemModal from '@/components/workspace/EditItemModal.vue';
import TransferModal from '@/components/workspace/TransferModal.vue';
import FilterTransactionsPopover from '@/components/workspace/FilterTransactionsPopover.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import Popover from '@/components/Popover.vue';
import { accounts, transactions, categories, transfers } from '@/lib/api/client';
import { emptyFilters, hasActiveFilters, type TransactionFilters } from '@/lib/utils/transactionFilters';
import { useAccountsStore } from '@/lib/stores/accounts';
import { Plus, CheckSquare, ReceiptText, Filter, Ellipsis } from '@lucide/vue';

const store = useAccountsStore();

const emit = defineEmits<{ (e: 'updated'): void }>();

const accountList = ref<any[]>([]);
const txnList = ref<any[]>([]);
const categoryList = ref<any[]>([]);
const loading = ref(true);

const filters = ref<TransactionFilters>(emptyFilters());
const page = ref(0);
const PAGE_SIZE = 20;
const totalCount = ref(0);
const pageToken = ref('');

const showAddModal = ref(false);
const showItemModal = ref(false);
const editingItem = ref<any>(null);
const showTransfer = ref(false);
const transferPrefill = ref<{ sourceId: string; fromId: string } | null>(null);
const showCheckboxes = ref(false);
const showFilter = ref(false);
const showMore = ref(false);
const confirmDelete = ref<{ ids: string[]; label: string } | null>(null);

const pagedTxns = computed(() => txnList.value);
const totalPages = computed(() => Math.max(1, Math.ceil(totalCount.value / PAGE_SIZE)));

async function loadPage() {
  loading.value = true;
  try {
    const resp = await transactions().listTransactions({
      pageSize: PAGE_SIZE,
      accountId: store.state.selectedAccountId || undefined,
      dateFrom: filters.value.dateFrom || undefined,
      dateTo: filters.value.dateTo || undefined,
      minAmount: filters.value.minAmount || undefined,
      maxAmount: filters.value.maxAmount || undefined,
      categoryId: filters.value.categoryId || undefined,
      name: filters.value.name || undefined,
      type: filters.value.type || undefined,
      ...(page.value > 0 && pageToken.value ? { pageToken: pageToken.value } : {})
    });
    txnList.value = (resp.transactions || []).map((t: any) => ({ ...t, type: t.type === 'CREDIT' ? 1 : 0 }));
    totalCount.value = resp.totalCount || txnList.value.length;
    pageToken.value = resp.nextPageToken || '';
  } catch (e) { console.error(e); }
  loading.value = false;
}

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

watch(filters, () => { page.value = 0; pageToken.value = ''; loadPage(); }, { deep: true });
watch(() => store.state.selectedAccountId, () => { page.value = 0; pageToken.value = ''; loadPage(); });
watch(page, () => { pageToken.value = ''; loadPage(); });

onMounted(() => { loadMeta(); loadPage(); });

function openAdd() {
  showAddModal.value = true;
}
function openEditItem(item: any) {
  editingItem.value = item; showItemModal.value = true;
}
function applyFilters(f: TransactionFilters) {
  filters.value = f;
  showFilter.value = false;
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
  <div class="space-y-5">
    <div v-if="loading" class="text-center py-16 text-subtle">Loading...</div>
    <div v-else-if="txnList.length === 0" class="flex items-center justify-center min-h-[40vh]">
      <div class="rounded-2xl border border-border bg-base-200 px-10 py-12 text-center max-w-md w-full">
        <div class="w-14 h-14 mx-auto rounded-2xl bg-primary-500/12 text-primary-400 flex items-center justify-center">
          <ReceiptText class="w-7 h-7" stroke-width="1.5" />
        </div>
        <h2 class="text-lg font-semibold text-text mt-4">Add your first transaction</h2>
        <p class="text-[13px] text-subtle mt-1.5">Record an expense or income to start building your spending history.</p>
        <button @click="openAdd" class="btn btn-primary btn-sm gap-1.5 mt-6">
          <Plus class="w-4 h-4" />
          Add Transaction
        </button>
      </div>
    </div>
    <div v-else class="card bg-base-200 border border-border">
      <div class="card-body p-6">
        <div class="flex items-center justify-between gap-2 mb-4">
          <h2 class="text-lg font-semibold text-text">Transactions</h2>
          <div class="flex items-center gap-2">
            <button @click="openAdd" class="btn btn-primary btn-sm gap-1.5">
              <Plus class="w-4 h-4" />
              Add
            </button>
            <div class="relative">
              <button
                @click="showFilter = !showFilter"
                class="btn btn-ghost btn-sm border border-track gap-1.5"
                :class="hasActiveFilters(filters) ? 'bg-primary-500/10 text-primary-400 border-primary-500/30' : 'text-text'"
              >
                <Filter class="w-3.5 h-3.5" />
                Filter
              </button>
              <FilterTransactionsPopover
                v-if="showFilter"
                :filters="filters"
                :categories="categoryList"
                @apply="applyFilters"
                @close="showFilter = false"
              />
            </div>
            <div class="relative">
              <button
                @click="showMore = !showMore"
                class="btn btn-ghost btn-sm border border-track"
                aria-label="More actions"
                title="More"
              >
                <Ellipsis class="w-4 h-4" />
              </button>
              <Popover v-if="showMore" panel-class="right-0 w-48" @close="showMore = false">
                <div class="py-1">
                  <button
                    @click="showCheckboxes = !showCheckboxes; showMore = false"
                    class="w-full flex items-center gap-3 px-4 py-2.5 text-[13px] font-medium text-left text-text-muted hover:text-primary-200 hover:bg-white/5 transition-colors"
                  >
                    <CheckSquare class="w-[18px] h-[18px]" stroke-width="1.5" />
                    {{ showCheckboxes ? 'Done selecting' : 'Select rows' }}
                  </button>
                </div>
              </Popover>
            </div>
          </div>
        </div>

        <TransactionsTab
          v-model:filters="filters"
          v-model:page="page"
          :paged-txns="pagedTxns"
          :filtered-count="totalCount"
          :total-pages="totalPages"
          :categories="categoryList"
          :all-txns="txnList"
          :accounts="accountList"
          v-model:show-checkboxes="showCheckboxes"
          @edit="openEditItem"
          @bulk-delete="requestBulkDelete"
          @transfer="openTransfer"
        />
      </div>
    </div>

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