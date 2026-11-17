<script setup lang="ts">
import { ref, onMounted } from 'vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import AppFolio from '@/components/ui/AppFolio.vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppEmpty from '@/components/ui/AppEmpty.vue';
import AppStamp from '@/components/ui/AppStamp.vue';
import AccountFormModal from '@/components/accounts/AccountFormModal.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { accounts } from '@/lib/api/client';
import { formatCurrency } from '@/lib/utils/format';
import { isCredit, accountTypeIcon } from '@/lib/utils/accountType';
import { Plus, Pencil, Trash2 } from '@lucide/vue';

const accountList = ref<any[]>([]);
const loading = ref(true);
const error = ref('');

const showModal = ref(false);
const editingAccount = ref<any>(null);
const confirmDelete = ref<any>(null);

async function loadAll() {
  loading.value = true;
  error.value = '';
  try {
    const resp = await accounts().listAccounts({});
    accountList.value = resp.accounts || [];
  } catch (e: any) { error.value = e?.message || 'Failed to load accounts'; }
  loading.value = false;
}

const TYPE_LABELS: Record<string, string> = { CURRENT: 'current', SAVINGS: 'savings', LOAN: 'loan', CREDIT_CARD: 'card' };
function typeLabel(type: string): string {
  return TYPE_LABELS[type] ?? String(type || '').replace(/_/g, ' ').toLowerCase();
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
  error.value = '';
  try {
    await accounts().deleteAccount({ id: confirmDelete.value.id });
    await loadAll();
  } catch (e: any) { error.value = e?.message || 'Failed to delete account'; }
  confirmDelete.value = null;
}

onMounted(loadAll);
</script>

<template>
  <div class="w-full flex flex-col gap-4">
    <SectionHeader title="Manage Accounts" subtitle="Add, edit or remove your accounts">
      <template #actions>
        <AppButton @click="openCreateAccount">
          <Plus class="h-[15px] w-[15px]" stroke-width="1.8" />
          Add account
        </AppButton>
      </template>
    </SectionHeader>

    <div v-if="error" class="rounded-card border border-debit/40 px-4 py-2.5 text-[13px] text-debit">{{ error }}</div>

    <AppFolio v-if="!loading && accountList.length" flush>
      <div class="flex items-center gap-3 border-b border-rule px-4 py-2 text-label font-medium uppercase tracking-[0.06em] text-ink-mute">
        <span class="w-7"></span>
        <span class="flex-1">Account</span>
        <span class="w-[120px]">Type</span>
        <span class="w-[74px]">Ending</span>
        <span class="w-[110px] text-right">Balance</span>
        <span class="w-[64px]"></span>
      </div>
      <div
        v-for="acc in accountList"
        :key="acc.id"
        class="group flex items-center gap-3 border-b border-rule-soft px-4 py-[9px] last:border-0 hover:bg-page-sunk/60"
      >
        <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-[4px] border border-rule bg-page-sunk text-ink-soft">
          <component :is="accountTypeIcon(acc.type)" class="h-[15px] w-[15px]" stroke-width="1.7" />
        </span>
        <span class="min-w-0 flex-1">
          <span class="block truncate text-[13px] text-ink">{{ acc.nickname || acc.bankName }}</span>
          <span class="block truncate text-[11px] text-ink-mute">{{ acc.bankName }}</span>
        </span>
        <span class="w-[120px]"><AppStamp :tone="isCredit(acc.type) ? 'debit' : 'neutral'">{{ typeLabel(acc.type) }}</AppStamp></span>
        <span class="tabular w-[74px] text-[12px] text-ink-soft">{{ acc.endingNumbers ? '···' + acc.endingNumbers : '—' }}</span>
        <span class="tabular w-[110px] text-right text-[13px] font-medium" :class="isCredit(acc.type) ? 'text-debit' : 'text-ink'">
          {{ isCredit(acc.type) ? '−' : '' }}{{ formatCurrency(Math.abs(acc.balance ?? 0)) }}
        </span>
        <span class="flex w-[64px] shrink-0 justify-end gap-0.5 text-ink-mute">
          <AppButton variant="icon" size="sm" label="Edit account" @click="openEditAccount(acc)">
            <Pencil class="h-[14px] w-[14px]" stroke-width="1.7" />
          </AppButton>
          <AppButton variant="icon" size="sm" label="Delete account" @click="requestDeleteAccount(acc)">
            <Trash2 class="h-[14px] w-[14px]" stroke-width="1.7" />
          </AppButton>
        </span>
      </div>
    </AppFolio>
    <AppFolio v-else-if="loading" flush>
      <div class="flex flex-col gap-2 p-4">
        <span v-for="i in 3" :key="i" class="block h-9 animate-pulse rounded-control bg-page-sunk" />
      </div>
    </AppFolio>
    <AppFolio v-else flush>
      <div class="px-4 py-8"><AppEmpty title="No accounts yet." hint="Add one to start booking entries.">
        <template #action><AppButton size="sm" @click="openCreateAccount">Add account</AppButton></template>
      </AppEmpty></div>
    </AppFolio>

    <AccountFormModal
      v-if="showModal"
      :account="editingAccount"
      @close="showModal = false"
      @saved="loadAll"
    />

    <ConfirmDialog
      v-if="confirmDelete"
      title="Delete account"
      :message="`Delete &quot;${confirmDelete.nickname || confirmDelete.bankName}&quot;? This can't be undone.`"
      @confirm="confirmDeleteNow"
      @cancel="confirmDelete = null"
    />
  </div>
</template>