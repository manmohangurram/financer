<script setup lang="ts">
import HScroll from '@/components/HScroll.vue';
import { formatCurrency } from '@/lib/utils/format';
import { isCredit } from '@/lib/utils/accountType';
import { useAccountsStore } from '@/lib/stores/accounts';

// The account filter: one rule-separated strip of chips, active one on paper-sunk.
defineProps<{ accounts: any[] }>();

const store = useAccountsStore();

function accountLabel(a: any) {
  return a.nickname || a.bankName;
}
</script>

<template>
  <HScroll :gap="6" class="flex min-h-[44px] items-center border-b border-rule pb-2">
    <button
      type="button"
      class="flex shrink-0 items-center gap-2 rounded-control px-2.5 py-1.5 text-[12.5px] font-medium transition-colors"
      :class="store.state.selectedAccountId === null ? 'bg-page-sunk text-ink' : 'text-ink-soft hover:bg-page-sunk/70 hover:text-ink'"
      @click="store.select(null)"
    >
      <span class="h-[7px] w-[7px] rounded-full" :class="store.state.selectedAccountId === null ? 'bg-accent' : 'bg-ink-faint'"></span>
      All accounts
    </button>
    <button
      v-for="acc in accounts"
      :key="acc.id"
      type="button"
      class="flex shrink-0 items-center gap-2 rounded-control px-2.5 py-1.5 text-[12.5px] font-medium transition-colors"
      :class="store.state.selectedAccountId === acc.id ? 'bg-page-sunk text-ink' : 'text-ink-soft hover:bg-page-sunk/70 hover:text-ink'"
      @click="store.select(acc.id)"
    >
      <span class="h-[7px] w-[7px] rounded-full" :class="isCredit(acc.type) ? 'bg-debit' : 'bg-credit'"></span>
      {{ accountLabel(acc) }}
      <span class="tabular" :class="store.state.selectedAccountId === acc.id ? 'text-ink' : 'text-ink-mute'">
        {{ formatCurrency(acc.balance ?? 0) }}
      </span>
    </button>
  </HScroll>
</template>
