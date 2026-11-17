<script setup lang="ts">
import { ref, onMounted } from 'vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import AppFolio from '@/components/ui/AppFolio.vue';
import AppButton from '@/components/ui/AppButton.vue';
import CategoriesTab from '@/components/workspace/CategoriesTab.vue';
import EditItemModal from '@/components/workspace/EditItemModal.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { categories, transactions } from '@/lib/api/client';
import { Plus } from '@lucide/vue';

const categoryList = ref<any[]>([]);
const txnList = ref<any[]>([]);
const loading = ref(true);
const error = ref('');

const showItemModal = ref(false);
const editingItem = ref<any>(null);
const confirmDelete = ref<{ ids: string[]; label: string } | null>(null);

async function loadAll() {
  loading.value = true;
  error.value = '';
  try {
    const [catResp, txnResp] = await Promise.all([
      categories().listCategories({ pageSize: 10000 }),
      transactions().listTransactions({ pageSize: 10000 })
    ]);
    categoryList.value = catResp.categories || [];
    txnList.value = (txnResp.transactions || []);
  } catch (e: any) { error.value = e?.message || 'Failed to load categories'; }
  loading.value = false;
}

function openCreateItem() {
  editingItem.value = null; showItemModal.value = true;
}
function openEditItem(item: any) {
  editingItem.value = item; showItemModal.value = true;
}
async function submitItem(payload: any) {
  error.value = '';
  try {
    if (editingItem.value) await categories().updateCategories({ categories: [{ id: editingItem.value.id, name: payload.name }] });
    else await categories().createCategories({ categories: [{ name: payload.name }] });
    showItemModal.value = false;
    await loadAll();
  } catch (e: any) { error.value = e?.message || 'Failed to save category'; }
}
function requestDeleteItem(item: any) {
  confirmDelete.value = { ids: [item.id], label: item.name };
}
async function confirmDeleteNow() {
  if (!confirmDelete.value) return;
  error.value = '';
  try {
    await categories().deleteCategories({ ids: confirmDelete.value.ids });
    showItemModal.value = false;
    await loadAll();
  } catch (e: any) { error.value = e?.message || 'Failed to delete category'; }
  confirmDelete.value = null;
}

onMounted(loadAll);
</script>

<template>
  <div class="w-full flex flex-col gap-4">
    <SectionHeader title="Categories" subtitle="Organise spending and set a monthly limit per category">
      <template #actions>
        <AppButton @click="openCreateItem">
          <Plus class="h-[15px] w-[15px]" stroke-width="1.8" />
          Add category
        </AppButton>
      </template>
    </SectionHeader>

    <div v-if="error" class="rounded-card border border-debit/40 px-4 py-2.5 text-[13px] text-debit">{{ error }}</div>

    <AppFolio v-if="!loading" class="min-h-0" :title="undefined" flush>
      <CategoriesTab :categories="categoryList" :txns="txnList" @edit="openEditItem" @delete="requestDeleteItem" @refresh="loadAll" />
    </AppFolio>
    <AppFolio v-else flush>
      <div class="flex flex-col gap-2 p-4">
        <span v-for="i in 5" :key="i" class="block h-9 animate-pulse rounded-control bg-page-sunk" />
      </div>
    </AppFolio>

    <EditItemModal
      v-if="showItemModal"
      :key="editingItem?.id || 'new'"
      type="category"
      :editing-item="editingItem"
      :accounts="[]"
      :categories="categoryList"
      default-account-id=""
      @close="showItemModal = false"
      @submit="submitItem"
      @delete="requestDeleteItem(editingItem)"
    />

    <ConfirmDialog
      v-if="confirmDelete"
      title="Delete category"
      :message="`Delete &quot;${confirmDelete.label}&quot;? This can't be undone.`"
      @confirm="confirmDeleteNow"
      @cancel="confirmDelete = null"
    />
  </div>
</template>