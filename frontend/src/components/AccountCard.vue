<script setup lang="ts">
import { formatCurrency } from '@/lib/utils/format';
import { accountTypeIcon, isCredit } from '@/lib/utils/accountType';
import { Pencil } from '@lucide/vue';

withDefaults(defineProps<{ account: any; compact?: boolean }>(), { compact: false });
const emit = defineEmits<{ (e: 'edit', account: any): void }>();
</script>

<template>
  <div
    class="card overflow-hidden rounded-card border border-rule bg-page transition-colors group hover:border-accent/40"
  >
    <div :class="compact ? 'p-3' : 'p-5'">
      <div class="flex items-center gap-4">
        <div
          class="flex shrink-0 items-center justify-center rounded-card"
          :class="[isCredit(account.type) ? 'bg-debit/10' : 'bg-credit/10', compact ? 'w-9 h-9' : 'w-12 h-12']"
        >
          <component
            :is="accountTypeIcon(account.type)"
            :class="compact ? 'w-5 h-5' : 'w-6 h-6'"
            stroke-width="2"
            aria-hidden="true"
          />
        </div>
        <div class="flex-1 min-w-0">
          <div class="truncate font-semibold text-ink" :class="compact ? 'text-[13.5px]' : 'text-[15px]'">
            {{ account.nickname || account.bankName }}
          </div>
          <div class="truncate text-ink-mute" :class="compact ? 'text-[11.5px]' : 'text-[13px]'">
            {{ account.bankName }}
          </div>
        </div>
        <button
          v-if="!compact"
          type="button"
          aria-label="Edit account"
          class="btn btn-ghost flex h-10 w-10 items-center justify-center rounded-control text-ink-mute shadow-none transition-colors hover:bg-accent/10 hover:text-accent focus-visible:opacity-100 lg:opacity-0 lg:group-hover:opacity-100"
          @click="emit('edit', account)"
        >
          <Pencil class="w-[18px] h-[18px]" aria-hidden="true" />
        </button>
      </div>
      <div :class="compact ? 'mt-2.5' : 'mt-4'">
        <div class="mb-0.5 text-ink-mute" :class="compact ? 'text-[11px]' : 'text-[12px]'">Balance</div>
        <div
          class="font-bold tracking-tight"
          :class="[isCredit(account.type) ? 'text-debit' : 'text-ink', compact ? 'text-[17px]' : 'text-[24px]']"
        >
          {{ isCredit(account.type) ? '-' : '' }}{{ formatCurrency(Math.abs(account.balance ?? 0)) }}
        </div>
      </div>
    </div>
  </div>
</template>
