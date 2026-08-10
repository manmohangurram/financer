<script setup lang="ts">
import { ref, onMounted } from 'vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import RulesTab from '@/components/workspace/RulesTab.vue';
import EditItemModal from '@/components/workspace/EditItemModal.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { categories, rules, accounts } from '@/lib/api/client';
import { Plus } from '@lucide/vue';

const ruleList = ref<any[]>([]);
const categoryList = ref<any[]>([]);
const accountList = ref<any[]>([]);
const loading = ref(true);
const error = ref('');

const showItemModal = ref(false);
const editingItem = ref<any>(null);
const confirmDelete = ref<{ ids: string[]; label: string } | null>(null);

async function loadAll() {
  loading.value = true;
  error.value = '';
  try {
    const [catResp, ruleResp, accResp] = await Promise.all([
      categories().listCategories({ pageSize: 10000 }),
      rules().listRules({}),
      accounts().listAccounts({})
    ]);
    categoryList.value = catResp.categories || [];
    ruleList.value = ruleResp.rules || [];
    accountList.value = accResp.accounts || [];
  } catch (e: any) { error.value = e?.message || 'Failed to load rules'; }
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
    if (editingItem.value) await rules().updateRule({ id: editingItem.value.id, ...payload });
    else await rules().createRule(payload);
    showItemModal.value = false;
    await loadAll();
  } catch (e: any) { error.value = e?.message || 'Failed to save rule'; }
}
function requestDeleteItem(item: any) {
  confirmDelete.value = { ids: [item.id], label: item.name };
}
async function confirmDeleteNow() {
  if (!confirmDelete.value) return;
  error.value = '';
  try {
    await rules().deleteRule({ id: confirmDelete.value.ids[0] });
    showItemModal.value = false;
    await loadAll();
  } catch (e: any) { error.value = e?.message || 'Failed to delete rule'; }
  confirmDelete.value = null;
}

onMounted(loadAll);
</script>

<template>
  <div class="w-full space-y-5">
    <SectionHeader title="Rules" subtitle="Automatically categorize transactions across all accounts">
      <template #actions>
        <button @click="openCreateItem" class="btn btn-primary btn-sm gap-1.5">
          <Plus class="w-4 h-4" />
          Add Rule
        </button>
      </template>
    </SectionHeader>

    <div v-if="error" class="rounded-xl border border-expense/30 bg-expense/10 px-4 py-3 text-[13px] text-expense">{{ error }}</div>

    <div v-if="loading" class="text-center py-16 text-subtle">Loading...</div>
    <div v-else class="card bg-base-200 border border-border">
      <div class="card-body p-6">
        <RulesTab :rules="ruleList" :categories="categoryList" :accounts="accountList" @edit="openEditItem" @refresh="loadAll" />
      </div>
    </div>

    <EditItemModal
      v-if="showItemModal"
      :key="editingItem?.id || 'new'"
      type="rule"
      :editing-item="editingItem"
      :accounts="accountList"
      :categories="categoryList"
      default-account-id=""
      @close="showItemModal = false"
      @submit="submitItem"
      @delete="requestDeleteItem(editingItem)"
    />

    <ConfirmDialog
      v-if="confirmDelete"
      title="Delete rule"
      :message="`Delete &quot;${confirmDelete.label}&quot;? This can't be undone.`"
      @confirm="confirmDeleteNow"
      @cancel="confirmDelete = null"
    />
  </div>
</template>