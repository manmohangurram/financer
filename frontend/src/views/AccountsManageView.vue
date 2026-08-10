<script setup lang="ts">
import { ref, onMounted } from 'vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import AccountFormModal from '@/components/accounts/AccountFormModal.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { accounts } from '@/lib/api/client';
import { formatCurrency } from '@/lib/utils/format';
import { isCredit, accountTypeIcon } from '@/lib/utils/accountType';
import { Plus, Pencil, Trash2 } from '@lucide/vue';

const accountList = ref<any[]>([]);
const loading = ref(true);

const showModal = ref(false);
const editingAccount = ref<any>(null);
const confirmDelete = ref<any>(null);

async function loadAll() {
  loading.value = true;
  try {
    const resp = await accounts().listAccounts({});
    accountList.value = resp.accounts || [];
  } catch (e) { console.error(e); }
  loading.value = false;
}

function openCreateAccount() {
  editingAccount.value = null;
  showModal.value = true;
}
function openEditAccount(acc: any) {
  editingAccount.value = acc;
  showModal.value = true;
}
function requestDeleteAccount(acc: any) { confirmDelete.value = acc; }
async function confirmDeleteNow() {
  if (!confirmDelete.value) return;
  try {
    await accounts().deleteAccount({ id: confirmDelete.value.id });
    await loadAll();
  } catch (e) { console.error(e); }
  confirmDelete.value = null;
}

onMounted(loadAll);
</script>

<template>
  <div class="w-full space-y-5">
    <SectionHeader title="Manage Accounts" subtitle="Add, edit or remove your accounts">
      <template #actions>
        <button @click="openCreateAccount" class="btn btn-primary btn-sm gap-1.5">
          <Plus class="w-4 h-4" />
          Add Account
        </button>
      </template>
    </SectionHeader>

    <div v-if="loading" class="text-center py-16 text-subtle">Loading...</div>
    <div v-else class="card bg-base-200 border border-border">
      <div class="card-body p-6">
        <div class="space-y-1.5">
          <div
            v-for="acc in accountList"
            :key="acc.id"
            class="flex items-center gap-3 px-4 py-3 rounded-xl bg-surface border border-border group"
          >
            <div
              class="w-10 h-10 rounded-xl flex items-center justify-center text-lg shrink-0"
              :class="isCredit(acc.type) ? 'bg-expense/12 text-expense' : 'bg-income/12 text-income'"
            >{{ accountTypeIcon(acc.type) }}</div>
            <div class="flex-1 min-w-0">
              <div class="text-[14px] text-text font-medium truncate">{{ acc.accountNickname || acc.bankName }}</div>
              <div class="text-[12px] text-subtle">{{ acc.bankName }}</div>
            </div>
            <span class="text-[15px] font-semibold shrink-0" :class="isCredit(acc.type) ? 'text-expense' : 'text-base-content'">
              {{ isCredit(acc.type) ? '-' : '' }}{{ formatCurrency(Math.abs(acc.balance ?? 0)) }}
            </span>
            <div class="flex gap-1 shrink-0">
              <button aria-label="Edit" @click="openEditAccount(acc)" class="p-2 rounded-lg text-subtle hover:text-primary-400 hover:bg-primary-500/10">
                <Pencil class="w-4 h-4" />
              </button>
              <button aria-label="Delete" @click="requestDeleteAccount(acc)" class="p-2 rounded-lg text-subtle hover:text-expense hover:bg-expense/10">
                <Trash2 class="w-4 h-4" />
              </button>
            </div>
          </div>
          <div v-if="accountList.length === 0" class="text-center py-16 text-subtle">No accounts yet</div>
        </div>
      </div>
    </div>

    <AccountFormModal
      v-if="showModal"
      :account="editingAccount"
      @close="showModal = false"
      @saved="loadAll"
    />

    <ConfirmDialog
      v-if="confirmDelete"
      title="Delete account"
      :message="`Delete &quot;${confirmDelete.accountNickname || confirmDelete.bankName}&quot;? This can't be undone.`"
      @confirm="confirmDeleteNow"
      @cancel="confirmDelete = null"
    />
  </div>
</template>