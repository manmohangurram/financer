<script setup lang="ts">
import { ref, computed } from 'vue';
import { formatCurrency } from '@/lib/utils/format';
import { ChartPie, ReceiptText } from '@lucide/vue';
import Pagination from '@/components/Pagination.vue';
import TransactionTable from '@/components/workspace/TransactionTable.vue';
import HScroll from '@/components/HScroll.vue';

const props = defineProps<{
  buckets: any[];
  categories: any[];
  selectedKey: string | null;
  drillTxns: any[];
  drillTotal: number;
  drillPage: number;
  drillSort: 'debit-desc' | 'debit-asc' | 'credit-desc' | 'credit-asc';
  categoriesList: any[];
  accountsList: any[];
}>();

const emit = defineEmits<{ (e: 'select-bucket', key: string): void; (e: 'drill-page', page: number): void; (e: 'sort-change', sort: 'debit-desc' | 'debit-asc' | 'credit-desc' | 'credit-asc'): void }>();

const sortDir = ref<'desc' | 'asc'>('desc');
const activeTab = ref<'transactions' | 'categories'>('transactions');
const hoveredPie = ref<{ id: string; part: 'debit' | 'credit' } | null>(null);
const sortedCats = computed(() => (sortDir.value === 'desc' ? props.categories : [...props.categories].reverse()));

const maxAmount = computed(() => Math.max(1, ...props.buckets.map((b) => b.amount)));
const yTicks = computed(() => [maxAmount.value, maxAmount.value * 0.75, maxAmount.value * 0.5, maxAmount.value * 0.25]);

function axisAmount(n: number) {
  if (n === 0) return '0';
  if (n >= 1000000) return `₹${(n / 1000000).toFixed(1)}M`;
  if (n >= 1000) return `₹${Math.round(n / 1000)}k`;
  return `₹${Math.round(n)}`;
}

// Donut wedge as an SVG path (viewBox 0 0 100 100, outer r=48, inner r=30).
function donutSegment(a0: number, a1: number): string {
  const cx = 50, cy = 50, ro = 48, ri = 30;
  const pt = (a: number, r: number) => {
    const rad = (a * Math.PI) / 180;
    return `${(cx + r * Math.sin(rad)).toFixed(2)} ${(cy - r * Math.cos(rad)).toFixed(2)}`;
  };
  const large = a1 - a0 > 180 ? 1 : 0;
  return `M ${pt(a0, ro)} A ${ro} ${ro} 0 ${large} 1 ${pt(a1, ro)} L ${pt(a1, ri)} A ${ri} ${ri} 0 ${large} 0 ${pt(a0, ri)} Z`;
}

// Full donut ring for a 0% or 100% single-color pie (evenodd hole).
function fullRing(): string {
  return 'M 50 2 A 48 48 0 1 1 50 98 A 48 48 0 1 1 50 2 Z M 50 20 A 30 30 0 1 0 50 80 A 30 30 0 1 0 50 20 Z';
}

function pieSlices(row: any): { part: 'debit' | 'credit'; d: string; fill: string; rule?: 'evenodd' }[] {
  const total = row.debit + row.credit;
  if (!total) {
    return [{ part: 'debit', d: fullRing(), fill: 'var(--color-track)', rule: 'evenodd' }];
  }
  const debitDeg = (row.debit / total) * 360;
  const slices: { part: 'debit' | 'credit'; d: string; fill: string; rule?: 'evenodd' }[] = [];
  if (debitDeg > 0.1) {
    slices.push({ part: 'debit', d: debitDeg >= 359.9 ? fullRing() : donutSegment(0, debitDeg), fill: 'var(--color-expense)', rule: debitDeg >= 359.9 ? 'evenodd' : undefined });
  }
  if (debitDeg < 359.9) {
    slices.push({ part: 'credit', d: debitDeg <= 0.1 ? fullRing() : donutSegment(debitDeg, 360), fill: 'var(--color-income)', rule: debitDeg <= 0.1 ? 'evenodd' : undefined });
  }
  return slices;
}

const txnSortOptions: { id: 'debit-desc' | 'debit-asc' | 'credit-desc' | 'credit-asc'; label: string }[] = [
  { id: 'debit-desc', label: 'Debit ↓' },
  { id: 'debit-asc', label: 'Debit ↑' },
  { id: 'credit-desc', label: 'Credit ↓' },
  { id: 'credit-asc', label: 'Credit ↑' }
];

const txnPages = computed(() => Math.max(1, Math.ceil(props.drillTotal / 10)));
</script>

<template>
  <div>
    <section aria-label="Spending bar chart">
      <div class="flex items-end gap-2">
        <div class="flex flex-col justify-between h-[200px] w-12 shrink-0 text-right text-[10px] text-subtle leading-none pr-1" aria-hidden="true">
          <span v-for="v in yTicks" :key="v" class="translate-y-1/3">{{ axisAmount(v) }}</span>
        </div>
        <HScroll class="flex-1" :gap="16" align="start">
          <div class="relative w-max">
            <div class="absolute inset-0 flex flex-col justify-between pointer-events-none" aria-hidden="true">
              <span v-for="v in yTicks" :key="'g' + v" class="w-full h-px bg-border/10"></span>
            </div>
            <div v-if="buckets.length" class="relative flex items-end gap-4 h-[200px]" role="group" aria-label="Spending by period">
              <button
                v-for="b in buckets"
                :key="b.key"
                type="button"
                class="group relative flex flex-col items-center justify-end gap-1.5 shrink-0 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/60"
                :aria-label="`${b.label}: ${b.amount ? formatCurrency(b.amount) : 'no spending'}`"
                :aria-pressed="selectedKey === b.key"
                @click="emit('select-bucket', b.key)"
              >
                <span
                  class="absolute -top-7 left-1/2 -translate-x-1/2 px-1.5 py-0.5 rounded-md bg-base-200 border border-border text-[11px] font-semibold text-text whitespace-nowrap pointer-events-none opacity-0 group-hover:opacity-100 transition-opacity z-10"
                >{{ formatCurrency(b.amount) }}</span>
                <span
                  class="w-12 rounded-md transition-all duration-200 border border-border"
                  :class="selectedKey === b.key ? 'bg-primary-500' : b.amount ? 'bg-primary-500/25 group-hover:bg-primary-500/45' : 'bg-white/[0.04]'"
                  :style="{ height: Math.max(b.amount ? 6 : 3, (b.amount / maxAmount) * 180) + 'px' }"
                ></span>
                <span class="text-[10px] leading-none text-subtle truncate max-w-12">{{ b.label }}</span>
              </button>
            </div>
            <div v-else class="min-w-[360px] h-[200px] flex items-center justify-center text-[12px] text-subtle">No spending in the selected range</div>
          </div>
        </HScroll>
      </div>
    </section>

    <div v-if="sortedCats.length || drillTxns.length" class="mt-6 rounded-xl border border-border bg-surface p-5">
      <div class="flex gap-1 rounded-xl bg-surface border border-border p-1 mb-5" role="group" aria-label="View">
        <button
          type="button"
          class="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg text-[13px] font-medium transition-colors"
          :class="activeTab === 'transactions' ? 'bg-primary-600 text-white' : 'text-subtle hover:text-text'"
          @click="activeTab = 'transactions'"
        >
          <ReceiptText class="w-4 h-4" stroke-width="1.5" />
          Transactions
        </button>
        <button
          type="button"
          class="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg text-[13px] font-medium transition-colors"
          :class="activeTab === 'categories' ? 'bg-primary-600 text-white' : 'text-subtle hover:text-text'"
          @click="activeTab = 'categories'"
        >
          <ChartPie class="w-4 h-4" stroke-width="1.5" />
          Categories
        </button>
      </div>

      <div class="min-h-[640px]">
        <div v-if="activeTab === 'transactions'">
        <div class="flex flex-wrap items-center justify-between gap-2 mb-3">
          <h3 class="text-[13px] font-semibold text-text">Transactions</h3>
          <div class="flex gap-1 rounded-xl bg-base-200 border border-border p-1" role="group" aria-label="Sort transactions">
            <button
              v-for="opt in txnSortOptions"
              :key="opt.id"
              type="button"
              class="px-2.5 py-1 rounded-lg text-[11px] font-medium transition-colors"
              :class="drillSort === opt.id ? 'bg-primary-600 text-white' : 'text-subtle hover:text-text'"
              @click="emit('sort-change', opt.id)"
            >{{ opt.label }}</button>
          </div>
        </div>
        <TransactionTable :transactions="drillTxns" :categories="categoriesList" :transfer-txns="drillTxns" :accounts="accountsList" :min-rows="10" readonly :empty-text="'No transactions in the selected range'" />
        <Pagination v-if="txnPages > 1" :page="drillPage" :total-pages="txnPages" :label="`${drillTotal} transactions`" @page-change="emit('drill-page', $event)" />
      </div>

      <div v-if="activeTab === 'categories' && sortedCats.length">
        <div class="flex flex-wrap items-center justify-between gap-2 mb-4">
          <h3 class="text-[13px] font-semibold text-text">Debit vs Credit by Category</h3>
          <div class="flex gap-1 rounded-xl bg-base-200 border border-border p-1" role="group" aria-label="Sort by net spend">
            <button
              type="button"
              class="px-2.5 py-1 rounded-lg text-[11px] font-medium transition-colors"
              :class="sortDir === 'desc' ? 'bg-primary-600 text-white' : 'text-subtle hover:text-text'"
              @click="sortDir = 'desc'"
            >Net: High → Low</button>
            <button
              type="button"
              class="px-2.5 py-1 rounded-lg text-[11px] font-medium transition-colors"
              :class="sortDir === 'asc' ? 'bg-primary-600 text-white' : 'text-subtle hover:text-text'"
              @click="sortDir = 'asc'"
            >Net: Low → High</button>
          </div>
        </div>

        <HScroll :gap="32" align="start">
          <div v-for="row in sortedCats" :key="row.id" class="flex flex-col items-center min-w-0 shrink-0">
            <div
              class="relative w-[166px] h-[166px] shrink-0"
              role="img"
              :aria-label="`${row.name}: debited ${formatCurrency(row.debit)}, credited ${formatCurrency(row.credit)}`"
              @mouseleave="hoveredPie = null"
            >
              <svg viewBox="0 0 100 100" class="w-full h-full">
                <path
                  v-for="s in pieSlices(row)"
                  :key="s.part"
                  :d="s.d"
                  :fill-rule="s.rule"
                  class="cursor-pointer"
                  :style="{ fill: s.fill }"
                  @mouseenter="hoveredPie = { id: row.id, part: s.part }"
                />
              </svg>
              <div class="absolute inset-0 flex flex-col items-center justify-center pointer-events-none px-1 text-center">
                <template v-if="hoveredPie && hoveredPie.id === row.id">
                  <span class="text-[9px] uppercase tracking-wider font-semibold" :class="hoveredPie.part === 'debit' ? 'text-expense' : 'text-income'">
                    {{ hoveredPie.part === 'debit' ? 'Debit' : 'Credit' }}
                  </span>
                  <span class="text-[11px] font-semibold text-text whitespace-nowrap">
                    {{ formatCurrency(hoveredPie.part === 'debit' ? row.debit : row.credit) }}
                  </span>
                </template>
                <template v-else>
                  <span class="text-[9px] text-subtle truncate max-w-full">{{ row.name }}</span>
                  <span class="text-[10px] font-bold text-text whitespace-nowrap">{{ formatCurrency(row.net) }}</span>
                </template>
              </div>
            </div>
          </div>
        </HScroll>

        <div class="overflow-x-auto mt-5">
          <table class="w-full">
            <thead>
              <tr class="text-[11px] text-subtle uppercase tracking-wider border-b border-border">
                <th class="text-left py-2 pr-3">Category</th>
                <th class="text-right py-2 px-3">Debited</th>
                <th class="text-right py-2 px-3">Credited</th>
                <th class="text-right py-2 pl-3">Net Spent</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border/50">
              <tr v-for="row in sortedCats" :key="row.id" class="text-[13px]">
                <td class="py-2 pr-3 text-text">{{ row.name }}</td>
                <td class="py-2 px-3 text-right text-expense">{{ formatCurrency(row.debit) }}</td>
                <td class="py-2 px-3 text-right text-income">{{ formatCurrency(row.credit) }}</td>
                <td class="py-2 pl-3 text-right font-semibold text-text">{{ formatCurrency(row.net) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        </div>
      </div>
    </div>
    <p v-else class="mt-6 text-center py-10 text-subtle">No spending in the selected range</p>
  </div>
</template>
