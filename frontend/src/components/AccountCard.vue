<script setup lang="ts">
import { formatCurrency } from '@/lib/utils/format';
import { accountTypeIcon, isCredit } from '@/lib/utils/accountType';
import { Pencil } from '@lucide/vue';

withDefaults(defineProps<{ account: any; compact?: boolean }>(), { compact: false });
const emit = defineEmits<{ (e: 'edit', account: any): void }>();
</script>

<template>
  <div class="card bg-base-300 border border-border overflow-hidden group hover:border-primary-500/40 transition-all">
    <div class="card-body" :class="compact ? 'p-3' : 'p-5'">
      <div class="flex items-center gap-4">
        <div
          class="rounded-xl flex items-center justify-center shrink-0"
          :class="[isCredit(account.type) ? 'bg-expense/12' : 'bg-income/12', compact ? 'w-9 h-9 text-lg' : 'w-12 h-12 text-2xl']"
        >
          {{ accountTypeIcon(account.type) }}
        </div>
        <div class="flex-1 min-w-0">
          <div class="font-semibold text-text truncate" :class="compact ? 'text-[13.5px]' : 'text-[15px]'">{{ account.accountNickname || account.bankName }}</div>
          <div class="text-subtle truncate" :class="compact ? 'text-[11.5px]' : 'text-[13px]'">{{ account.bankName }}</div>
        </div>
        <button
          v-if="!compact"
          @click="emit('edit', account)"
          class="w-9 h-9 rounded-lg flex items-center justify-center text-subtle hover:text-primary-400 hover:bg-primary-500/10 transition-all opacity-0 group-hover:opacity-100"
          aria-label="Edit account"
        >
          <Pencil class="w-[18px] h-[18px]" />
        </button>
      </div>
      <div :class="compact ? 'mt-2.5' : 'mt-4'">
        <div class="text-subtle mb-0.5" :class="compact ? 'text-[11px]' : 'text-[12px]'">Balance</div>
        <div class="font-bold tracking-tight" :class="[isCredit(account.type) ? 'text-expense' : 'text-base-content', compact ? 'text-[17px]' : 'text-[24px]']">
          {{ isCredit(account.type) ? '-' : '' }}{{ formatCurrency(Math.abs(account.balance ?? 0)) }}
        </div>
      </div>
    </div>
  </div>
</template>
