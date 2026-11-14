<script setup lang="ts">
import { ref, onMounted } from 'vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import AppFolio from '@/components/ui/AppFolio.vue';
import AppButton from '@/components/ui/AppButton.vue';
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
  <div class="w-full flex flex-col gap-4">
    <SectionHeader title="Rules" subtitle="Rename, categorise or link entries as they are written">
      <template #actions>
        <AppButton @click="openCreateItem">
          <Plus class="h-[15px] w-[15px]" stroke-width="1.8" />
          Add rule
        </AppButton>
      </template>
    </SectionHeader>

    <div v-if="error" class="rounded-card border border-debit/40 px-4 py-2.5 text-[13px] text-debit">{{ error }}</div>

    <AppFolio v-if="!loading" flush>
      <RulesTab :rules="ruleList" :categories="categoryList" :accounts="accountList" @edit="openEditItem" @refresh="loadAll" @create="openCreateItem" />
    </AppFolio>
    <AppFolio v-else flush>
      <div class="flex flex-col gap-2 p-4">
        <span v-for="i in 4" :key="i" class="block h-9 animate-pulse rounded-control bg-page-sunk" />
      </div>
    </AppFolio>

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