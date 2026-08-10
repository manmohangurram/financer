<script setup lang="ts">
import { ref, onMounted } from 'vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import CategoriesTab from '@/components/workspace/CategoriesTab.vue';
import EditItemModal from '@/components/workspace/EditItemModal.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { categories, transactions } from '@/lib/api/client';
import { Plus } from '@lucide/vue';

const categoryList = ref<any[]>([]);
const txnList = ref<any[]>([]);
const loading = ref(true);

const showItemModal = ref(false);
const editingItem = ref<any>(null);
const confirmDelete = ref<{ ids: string[]; label: string } | null>(null);

async function loadAll() {
  loading.value = true;
  try {
    const [catResp, txnResp] = await Promise.all([
      categories().listCategories({ pageSize: 10000 }),
      transactions().listTransactions({ pageSize: 10000 })
    ]);
    categoryList.value = catResp.categories || [];
    txnList.value = (txnResp.transactions || []);
  } catch (e) { console.error(e); }
  loading.value = false;
}

function openCreateItem() {
  editingItem.value = null; showItemModal.value = true;
}
function openEditItem(item: any) {
  editingItem.value = item; showItemModal.value = true;
}
async function submitItem(payload: any) {
  try {
    if (editingItem.value) await categories().updateCategories({ categories: [{ id: editingItem.value.id, name: payload.name }] });
    else await categories().createCategories({ categories: [{ name: payload.name }] });
    showItemModal.value = false;
    await loadAll();
  } catch (e) { console.error(e); }
}
function requestDeleteItem(item: any) {
  confirmDelete.value = { ids: [item.id], label: item.name };
}
async function confirmDeleteNow() {
  if (!confirmDelete.value) return;
  try {
    await categories().deleteCategories({ ids: confirmDelete.value.ids });
    showItemModal.value = false;
    await loadAll();
  } catch (e) { console.error(e); }
  confirmDelete.value = null;
}

onMounted(loadAll);
</script>

<template>
  <div class="w-full space-y-5">
    <SectionHeader title="Categories" subtitle="Organize spending into categories">
      <template #actions>
        <button @click="openCreateItem" class="btn btn-primary btn-sm gap-1.5">
          <Plus class="w-4 h-4" />
          Add Category
        </button>
      </template>
    </SectionHeader>

    <div v-if="loading" class="text-center py-16 text-subtle">Loading...</div>
    <div v-else class="card bg-base-200 border border-border">
      <div class="card-body p-6">
        <CategoriesTab :categories="categoryList" :txns="txnList" @edit="openEditItem" @delete="requestDeleteItem" />
      </div>
    </div>

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