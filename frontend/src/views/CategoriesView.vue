<script setup lang="ts">
import { ref, onMounted } from 'vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import AppFolio from '@/components/ui/AppFolio.vue';
import AppButton from '@/components/ui/AppButton.vue';
import CategoriesTab from '@/components/workspace/CategoriesTab.vue';
import EditItemModal from '@/components/workspace/EditItemModal.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { categories } from '@/lib/api/client';
import { Plus } from '@lucide/vue';

const categoryList = ref<any[]>([]);
const loading = ref(true);
const error = ref('');

const showItemModal = ref(false);
const editingItem = ref<any>(null);
const confirmDelete = ref<{ ids: string[]; label: string } | null>(null);

async function loadAll() {
  loading.value = true;
  error.value = '';
  try {
    const catResp = await categories().listCategories({ pageSize: 10000 });
    categoryList.value = catResp.categories || [];
  } catch (e: any) {
    error.value = e?.message || 'Failed to load categories';
  }
  loading.value = false;
}

function openCreateItem() {
  editingItem.value = null;
  showItemModal.value = true;
}
function openEditItem(item: any) {
  editingItem.value = item;
  showItemModal.value = true;
}

async function saveBudget(id: string, raw: string | number | undefined) {
  const value = raw === '' || raw === undefined ? 0 : Number(raw);
  if (!Number.isFinite(value) || value < 0) throw new Error('Monthly budget must be zero or a positive amount.');
  await categories().setCategoryBudget({ id, monthlyBudget: value });
}

async function submitItem(payload: any) {
  error.value = '';
  try {
    const name = String(payload.name || '').trim();
    if (!name) throw new Error('Name is required');
    if (editingItem.value) {
      await categories().updateCategories({ categories: [{ id: editingItem.value.id, name }] });
      await saveBudget(editingItem.value.id, payload.monthlyBudget);
    } else {
      await categories().createCategories({ categories: [{ name }] });
      // The bulk create endpoint returns no ids, so resolve the new row by name.
      const resp = await categories().listCategories({ pageSize: 10000 });
      const created = (resp.categories || []).find((c: any) => c.name === name);
      if (created) await saveBudget(created.id, payload.monthlyBudget);
    }
    showItemModal.value = false;
    await loadAll();
  } catch (e: any) {
    error.value = e?.message || 'Failed to save category';
  }
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
  } catch (e: any) {
    error.value = e?.message || 'Failed to delete category';
  }
  confirmDelete.value = null;
}

onMounted(loadAll);
</script>

<template>
  <div class="w-full flex flex-col gap-4">
    <SectionHeader title="Categories" subtitle="Organise spending and set a monthly budget per category">
      <template #actions>
        <AppButton @click="openCreateItem">
          <Plus class="h-[15px] w-[15px]" stroke-width="1.8" />
          Add category
        </AppButton>
      </template>
    </SectionHeader>

    <div v-if="error" class="rounded-card border border-debit/40 px-4 py-2.5 text-[13px] text-debit">{{ error }}</div>

    <AppFolio v-if="!loading" class="min-h-0" :title="undefined" flush>
      <CategoriesTab :categories="categoryList" @edit="openEditItem" @delete="requestDeleteItem" />
    </AppFolio>
    <AppFolio v-else flush>
      <div class="flex flex-col gap-2 p-4">
        <span v-for="i in 4" :key="i" class="block h-9 animate-pulse rounded-control bg-page-sunk" />
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
      :message="`Delete “${confirmDelete.label}”? Transactions keep their history but lose this category.`"
      @confirm="confirmDeleteNow"
      @cancel="confirmDelete = null"
    />
  </div>
</template>
