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
import { SlidersHorizontal, Tag, Settings2, Landmark, Plus } from '@lucide/vue';

const router = useRouter();

const accountList = ref<any[]>([]);
const showAccountModal = ref(false);

const totalCash = computed(() =>
  accountList.value.filter((a: any) => !isCredit(a.accountType)).reduce((s: number, a: any) => s + (a.balance ?? 0), 0)
);
const creditOwed = computed(() =>
  accountList.value.filter((a: any) => isCredit(a.accountType)).reduce((s: number, a: any) => s + Math.abs(a.balance ?? 0), 0)
);

async function loadAll() {
  try {
    const accResp = await accounts().listAccounts({});
    accountList.value = accResp.accounts || [];
  } catch (e) { console.error(e); }
}

onMounted(loadAll);
</script>

<template>
  <div class="w-full space-y-5">
    <div class="flex items-center justify-between gap-4">
      <div>
        <h1 class="text-[28px] font-bold text-white tracking-tight">Accounts</h1>
        <p class="text-[14px] text-subtle mt-0.5">Track spending & transactions across your accounts</p>
      </div>
      <div v-if="accountList.length > 0" class="flex flex-wrap items-center gap-2 shrink-0">
        <button
          class="btn btn-ghost btn-sm border border-track gap-1.5"
          @click="router.push('/accounts/rules')"
          aria-label="Rules"
        >
          <SlidersHorizontal class="w-4 h-4" />
          Rules
        </button>
        <button
          class="btn btn-ghost btn-sm border border-track gap-1.5"
          @click="router.push('/accounts/categories')"
          aria-label="Categories"
        >
          <Tag class="w-4 h-4" />
          Categories
        </button>
        <button
          class="btn btn-ghost btn-sm border border-track gap-1.5"
          @click="router.push('/accounts/manage')"
          aria-label="Manage accounts"
        >
          <Settings2 class="w-4 h-4" />
          Manage
        </button>
      </div>
    </div>

    <div v-if="accountList.length === 0" class="flex items-center justify-center min-h-[calc(100vh-180px)]">
      <div class="rounded-2xl border border-border bg-base-200 px-10 py-12 text-center max-w-md w-full">
        <div class="w-14 h-14 mx-auto rounded-2xl bg-primary-500/12 text-primary-400 flex items-center justify-center">
          <Landmark class="w-7 h-7" stroke-width="1.5" />
        </div>
        <h2 class="text-lg font-semibold text-text mt-4">Add your first account</h2>
        <p class="text-[13px] text-subtle mt-1.5">Start tracking your money — add a bank account to begin recording transactions.</p>
        <button class="btn btn-primary btn-sm gap-1.5 mt-6" @click="showAccountModal = true">
          <Plus class="w-4 h-4" />
          Add Account
        </button>
      </div>
    </div>

    <template v-else>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <StatCard label="Total Cash" :value="formatCurrency(totalCash)" value-class="text-income" />
        <StatCard label="Credit Owed" :value="formatCurrency(creditOwed)" value-class="text-expense" />
        <StatCard label="Net Worth" :value="formatCurrency(totalCash - creditOwed)" />
      </div>

      <div class="flex items-center gap-3">
        <div class="flex-1 min-w-0">
          <AccountFilterBar :accounts="accountList" />
        </div>
      </div>

      <TransactionsPanel />
    </template>

    <AccountFormModal v-if="showAccountModal" :account="null" @close="showAccountModal = false" @saved="loadAll" />
  </div>
</template>