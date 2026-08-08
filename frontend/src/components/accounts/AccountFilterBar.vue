<script setup lang="ts">
import { formatCurrency } from '@/lib/utils/format';
import { isCredit } from '@/lib/utils/accountType';
import { useAccountsStore } from '@/lib/stores/accounts';

defineProps<{ accounts: any[] }>();

const store = useAccountsStore();

function accountLabel(a: any) {
  return a.accountNickname || a.bankName;
}
</script>

<template>
  <div class="rounded-2xl border border-border bg-base-200 min-h-[60px] flex items-center px-3">
    <div class="flex flex-wrap items-center gap-1.5">
      <button
        @click="store.select(null)"
        class="px-3 py-1.5 rounded-xl text-[13.5px] font-medium transition-all flex items-center gap-2"
        :class="store.state.selectedAccountId === null ? 'bg-primary-500 text-white' : 'bg-surface text-text-muted border border-border hover:border-primary-500/40'"
      >
        <span class="w-2 h-2 rounded-full" :class="store.state.selectedAccountId === null ? 'bg-white' : 'bg-track'"></span>
        All Accounts
      </button>
      <button
        v-for="acc in accounts"
        :key="acc.id"
        @click="store.select(acc.id)"
        class="px-3 py-1.5 rounded-xl text-[13.5px] font-medium transition-all flex items-center gap-2"
        :class="store.state.selectedAccountId === acc.id ? 'bg-primary-500 text-white' : 'bg-surface text-text-muted border border-border hover:border-primary-500/40'"
      >
        <span class="w-2 h-2 rounded-full" :class="store.state.selectedAccountId === acc.id ? 'bg-white' : isCredit(acc.accountType) ? 'bg-expense' : 'bg-income'"></span>
        {{ accountLabel(acc) }}
        <span class="font-semibold" :class="store.state.selectedAccountId === acc.id ? 'text-white' : isCredit(acc.accountType) ? 'text-expense' : 'text-income'">
          {{ formatCurrency(acc.balance ?? 0) }}
        </span>
      </button>
    </div>
    <p v-if="accounts.length === 0" class="text-center py-4 text-subtle text-[13px]">No accounts yet. Add one to start tracking.</p>
  </div>
</template>