<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import StatCard from '@/components/StatCard.vue';
import AccountFilterBar from '@/components/accounts/AccountFilterBar.vue';
import AccountFormModal from '@/components/accounts/AccountFormModal.vue';
import TransactionsPanel from '@/components/workspace/TransactionsPanel.vue';
import { accounts } from '@/lib/api/client';
import { formatCurrency } from '@/lib/utils/format';
import { isCredit } from '@/lib/utils/accountType';
import { Settings2, Landmark, Plus } from '@lucide/vue';

const router = useRouter();

const accountList = ref<any[]>([]);
const showAccountModal = ref(false);
const loadError = ref('');

const totalCash = computed(() =>
  accountList.value.filter((a: any) => !isCredit(a.type)).reduce((s: number, a: any) => s + (a.balance ?? 0), 0)
);
const creditOwed = computed(() =>
  accountList.value.filter((a: any) => isCredit(a.type)).reduce((s: number, a: any) => s + Math.abs(a.balance ?? 0), 0)
);

async function loadAll() {
  loadError.value = '';
  try {
    const accResp = await accounts().listAccounts({});
    accountList.value = accResp.accounts || [];
  } catch (e: any) { loadError.value = e?.message || 'Failed to load accounts'; }
}

onMounted(loadAll);
</script>

<template>
  <div class="w-full flex h-full min-h-0 flex-col gap-4">
    <div class="flex items-center justify-between gap-4">
      <div>
        <h1 class="text-[22px] leading-none text-ink">Accounts</h1>
        <p class="hidden md:block text-[14px] text-ink-mute mt-0.5">Track spending & transactions across your accounts</p>
      </div>
      <div v-if="accountList.length > 0" class="flex flex-wrap items-center gap-2 shrink-0">
        <button
          class="ns-quiet border border-rule gap-1.5"
          @click="router.push('/accounts/manage')"
          aria-label="Manage accounts"
        >
          <Settings2 class="w-4 h-4" />
          Manage
        </button>
      </div>
    </div>

    <div v-if="loadError" class="rounded-card border border-expense/30 bg-debit/10 px-4 py-3 text-[13px] text-debit">{{ loadError }}</div>

    <div v-if="accountList.length === 0" class="flex items-center justify-center min-h-[calc(100vh-180px)]">
      <div class="rounded-card border border-rule bg-page px-10 py-12 text-center max-w-md w-full">
        <div class="w-14 h-14 mx-auto rounded-card bg-accent/12 text-accent flex items-center justify-center">
          <Landmark class="w-7 h-7" stroke-width="1.5" />
        </div>
        <h2 class="text-lg font-semibold text-ink mt-4">Add your first account</h2>
        <p class="text-[13px] text-ink-mute mt-1.5">Start tracking your money — add a bank account to begin recording transactions.</p>
        <button class="ns-primary gap-1.5 mt-6" @click="showAccountModal = true">
          <Plus class="w-4 h-4" />
          Add Account
        </button>
      </div>
    </div>

    <template v-else>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <StatCard label="Total Cash" :value="formatCurrency(totalCash)" value-class="text-credit" />
        <StatCard label="Credit Owed" :value="formatCurrency(creditOwed)" value-class="text-debit" />
        <StatCard label="Net Worth" :value="formatCurrency(totalCash - creditOwed)" />
      </div>

      <AccountFilterBar :accounts="accountList" />

      <TransactionsPanel @updated="loadAll" />
    </template>

    <AccountFormModal v-if="showAccountModal" :account="null" @close="showAccountModal = false" @saved="loadAll" />
  </div>
</template>