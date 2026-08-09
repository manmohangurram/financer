<script setup lang="ts">
import StatCard from '@/components/StatCard.vue';
import { formatCurrency } from '@/lib/utils/format';

defineProps<{
  totalInvested: number;
  totalCurrentValue: number;
  totalUnrealizedPnl: number;
}>();
</script>

<template>
  <div class="grid grid-cols-2 lg:grid-cols-3 gap-4">
    <StatCard label="Total Invested" :value="formatCurrency(totalInvested)" />
    <StatCard label="Current Value" :value="formatCurrency(totalCurrentValue)" value-class="text-income" />
    <StatCard
      label="P&L"
      :value="formatCurrency(totalUnrealizedPnl)"
      :value-class="totalUnrealizedPnl >= 0 ? 'text-income' : 'text-expense'"
    />
  </div>
</template>