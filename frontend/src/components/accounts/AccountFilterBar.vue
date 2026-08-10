<script setup lang="ts">
import HScroll from '@/components/HScroll.vue';
import { formatCurrency } from '@/lib/utils/format';
import { isCredit } from '@/lib/utils/accountType';
import { useAccountsStore } from '@/lib/stores/accounts';

defineProps<{ accounts: any[] }>();

const store = useAccountsStore();

function accountLabel(a: any) {
  return a.nickname || a.bankName;
}
</script>

<template>
  <HScroll :gap="6" class="rounded-2xl border border-border bg-base-200 min-h-[60px] flex items-center px-3 overflow-hidden">
    <button
      @click="store.select(null)"
      class="shrink-0 px-3 py-1.5 rounded-xl text-[13.5px] font-medium transition-all flex items-center gap-2"
      :class="store.state.selectedAccountId === null ? 'bg-primary-600 text-white' : 'bg-surface text-text-muted border border-border hover:border-primary-500/40'"
    >
      <span class="w-2 h-2 rounded-full" :class="store.state.selectedAccountId === null ? 'bg-white' : 'bg-track'"></span>
      All Accounts
    </button>
    <button
      v-for="acc in accounts"
      :key="acc.id"
      @click="store.select(acc.id)"
      class="shrink-0 px-3 py-1.5 rounded-xl text-[13.5px] font-medium transition-all flex items-center gap-2"
      :class="store.state.selectedAccountId === acc.id ? 'bg-primary-600 text-white' : 'bg-surface text-text-muted border border-border hover:border-primary-500/40'"
    >
      <span class="w-2 h-2 rounded-full" :class="store.state.selectedAccountId === acc.id ? 'bg-white' : isCredit(acc.type) ? 'bg-expense' : 'bg-income'"></span>
      {{ accountLabel(acc) }}
      <span class="font-semibold" :class="store.state.selectedAccountId === acc.id ? 'text-white' : isCredit(acc.type) ? 'text-expense' : 'text-income'">
        {{ formatCurrency(acc.balance ?? 0) }}
      </span>
    </button>
  </HScroll>
</template>
