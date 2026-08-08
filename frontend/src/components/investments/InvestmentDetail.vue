<script setup lang="ts">
import { formatCurrency } from '@/lib/utils/format';
import PriceHistoryChart from '@/components/investments/PriceHistoryChart.vue';
import { ArrowLeft } from '@lucide/vue';

defineProps<{ investment: any; lots: any[] }>();
const emit = defineEmits<{
  (e: 'close'): void;
  (e: 'edit'): void;
  (e: 'add-buy'): void;
  (e: 'add-sell'): void;
  (e: 'delete-lot', lot: any): void;
  (e: 'delete-investment'): void;
}>();

function pnlClass(v: number): string {
  return v >= 0 ? 'text-income' : 'text-expense';
}

function typeLabel(t: string): string {
  return t === 'INVESTMENT_TYPE_MUTUAL_FUND' ? 'MUTUAL FUND' : 'STOCK';
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-start justify-between gap-4">
      <div class="flex items-center gap-3 min-w-0">
        <button
          class="btn btn-ghost btn-sm btn-circle border border-track shrink-0"
          aria-label="Back to investments"
          @click="emit('close')"
        >
          <ArrowLeft class="w-4 h-4" />
        </button>
        <div class="min-w-0">
          <div class="flex items-center gap-3">
            <h2 class="text-[22px] font-bold text-base-content tracking-tight truncate">{{ investment.name }}</h2>
            <span class="badge badge-outline badge-sm border-border text-subtle shrink-0">{{ typeLabel(investment.investmentType) }}</span>
          </div>
          <div v-if="investment.symbol" class="text-[13px] text-subtle mt-0.5">{{ investment.symbol }}</div>
          <div v-else class="text-[13px] text-subtle mt-0.5">Manual NAV only</div>
        </div>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <button class="btn btn-outline btn-error btn-sm text-[13px]" @click="emit('delete-investment')">Delete</button>
        <button class="btn btn-ghost btn-sm border border-track" @click="emit('edit')">Edit</button>
      </div>
    </div>

    <div class="grid grid-cols-2 lg:grid-cols-5 gap-4">
      <div class="rounded-xl border border-border bg-base-200 p-4">
        <div class="text-[11px] uppercase tracking-wide text-faint">Quantity</div>
        <div class="text-[18px] font-bold text-text mt-1">{{ investment.quantity }}</div>
      </div>
      <div class="rounded-xl border border-border bg-base-200 p-4">
        <div class="text-[11px] uppercase tracking-wide text-faint">Avg Cost</div>
        <div class="text-[18px] font-bold text-text mt-1">{{ formatCurrency(investment.avgCost) }}</div>
      </div>
      <div class="rounded-xl border border-border bg-base-200 p-4">
        <div class="text-[11px] uppercase tracking-wide text-faint">Current Price</div>
        <div class="text-[18px] font-bold text-text mt-1">{{ formatCurrency(investment.currentPrice) }}</div>
      </div>
      <div class="rounded-xl border border-border bg-base-200 p-4">
        <div class="text-[11px] uppercase tracking-wide text-faint">Current Value</div>
        <div class="text-[18px] font-bold text-income mt-1">{{ formatCurrency(investment.currentValue) }}</div>
      </div>
      <div class="rounded-xl border border-border bg-base-200 p-4">
        <div class="text-[11px] uppercase tracking-wide text-faint">Realized P&L</div>
        <div class="text-[18px] font-bold mt-1" :class="pnlClass(investment.realizedPnl)">{{ formatCurrency(investment.realizedPnl) }}</div>
      </div>
    </div>

    <PriceHistoryChart :investment="investment" />

    <div class="flex items-center justify-between">
      <h3 class="text-[15px] font-semibold text-text">Lot history</h3>
      <div class="flex items-center gap-2">
        <button class="btn btn-success btn-sm text-[13px]" @click="emit('add-buy')">+ Buy</button>
        <button class="btn btn-ghost btn-sm border border-track text-[13px] border-border text-text-muted" @click="emit('add-sell')">- Sell</button>
      </div>
    </div>

    <div class="overflow-x-auto bg-base-200 border border-border rounded-2xl">
      <table class="table w-full">
        <thead>
          <tr class="text-[11px] uppercase tracking-wide text-faint">
            <th class="pl-6">Date</th>
            <th>Side</th>
            <th class="text-right">Quantity</th>
            <th class="text-right">Price</th>
            <th class="text-right pr-6"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="lot in lots" :key="lot.id">
            <td class="pl-6 text-text-secondary">{{ new Date(lot.occurredAt).toLocaleDateString('en-IN') }}</td>
            <td>
              <span class="badge badge-sm" :class="lot.side === 1 ? 'bg-income/15 text-income border-income/30' : 'bg-expense/15 text-expense border-expense/30'">
                {{ lot.side === 1 ? 'BUY' : 'SELL' }}
              </span>
            </td>
            <td class="text-right text-text-secondary">{{ lot.quantity }}</td>
            <td class="text-right text-text-secondary">{{ formatCurrency(lot.price) }}</td>
            <td class="text-right pr-6">
              <button class="btn btn-ghost btn-xs text-expense" @click="emit('delete-lot', lot)">Delete</button>
            </td>
          </tr>
          <tr v-if="lots.length === 0">
            <td colspan="5" class="pl-6 py-8 text-center text-subtle">No lots yet — add your first buy.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>