<script setup lang="ts">
import { formatCurrency } from '@/lib/utils/format';

defineProps<{ investments: any[] }>();
const emit = defineEmits<{ (e: 'select', investment: any): void }>();

function pnlClass(v: number): string {
  return v >= 0 ? 'text-income' : 'text-expense';
}

function typeLabel(t: string): string {
  return t === 'MUTUAL_FUND' ? 'MUTUAL FUND' : 'STOCK';
}
</script>

<template>
  <div class="overflow-x-auto bg-base-200 border border-border rounded-2xl shadow-sm">
    <table class="table w-full">
      <thead>
        <tr class="text-[11px] uppercase tracking-wide text-faint">
          <th class="pl-6">Name</th>
          <th>Type</th>
          <th class="text-right">Quantity</th>
          <th class="text-right">Current Price</th>
          <th class="text-right">Invested</th>
          <th class="text-right">Current Value</th>
          <th class="text-right pr-6">P&L</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="inst in investments"
          :key="inst.id"
          class="cursor-pointer hover:bg-white/[0.03] transition-colors"
          @click="emit('select', inst)"
        >
          <td class="pl-6">
            <div class="font-semibold text-text">{{ inst.name }}</div>
            <div v-if="inst.symbol" class="text-[12px] text-subtle">{{ inst.symbol }}</div>
          </td>
          <td><span class="badge badge-outline badge-sm border-border text-subtle">{{ typeLabel(inst.investmentType) }}</span></td>
          <td class="text-right text-text-secondary">{{ inst.quantity }}</td>
          <td class="text-right text-text-secondary">{{ formatCurrency(inst.currentPrice) }}</td>
          <td class="text-right text-text-secondary">{{ formatCurrency(inst.avgCost * inst.quantity) }}</td>
          <td class="text-right font-semibold text-text">{{ formatCurrency(inst.currentValue) }}</td>
          <td class="text-right pr-6" :class="pnlClass(inst.unrealizedPnl)">{{ formatCurrency(inst.unrealizedPnl) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>