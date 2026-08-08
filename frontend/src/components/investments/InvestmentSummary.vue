<script setup lang="ts">
import StatCard from '@/components/StatCard.vue';
import { formatCurrency } from '@/lib/utils/format';

defineProps<{
  totalInvested: number;
  totalCurrentValue: number;
  totalUnrealizedPnl: number;
  totalRealizedPnl: number;
}>();
</script>

<template>
  <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
    <StatCard label="Total Invested" :value="formatCurrency(totalInvested)" />
    <StatCard label="Current Value" :value="formatCurrency(totalCurrentValue)" value-class="text-income" />
    <StatCard
      label="Unrealized P&L"
      :value="formatCurrency(totalUnrealizedPnl)"
      :value-class="totalUnrealizedPnl >= 0 ? 'text-income' : 'text-expense'"
    />
    <StatCard
      label="Realized P&L"
      :value="formatCurrency(totalRealizedPnl)"
      :value-class="totalRealizedPnl >= 0 ? 'text-income' : 'text-expense'"
    />
  </div>
</template>