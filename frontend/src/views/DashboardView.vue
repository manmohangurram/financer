<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import StatCard from '@/components/StatCard.vue';
import AccountCard from '@/components/AccountCard.vue';
import { analytics } from '@/lib/api/client';
import { formatCurrency } from '@/lib/utils/format';

const data = ref({ totalBalance: 0, totalIncome: 0, totalExpenses: 0, portfolioValue: 0, accounts: [] as any[], investments: [] as any[] });
const loading = ref(true);

const totalBalance = computed(() => data.value.totalBalance);
const totalIncome = computed(() => data.value.totalIncome);
const totalExpenses = computed(() => data.value.totalExpenses);
const accountList = computed(() => data.value.accounts);
const portfolioValue = computed(() => data.value.portfolioValue);

const portfolioByPnl = computed(() =>
  [...data.value.investments].sort(
    (a: any, b: any) => (b.unrealizedPnl || 0) - (a.unrealizedPnl || 0)
  )
);

async function loadData() {
  loading.value = true;
  try {
    const resp = await analytics().dashboard({});
    data.value = {
      totalBalance: resp.totalBalance || 0,
      totalIncome: resp.totalIncome || 0,
      totalExpenses: resp.totalExpenses || 0,
      portfolioValue: resp.portfolioValue || 0,
      accounts: resp.accounts || [],
      investments: resp.investments || []
    };
  } catch (e) {
    console.error('Failed to load dashboard:', e);
  }
  loading.value = false;
}

onMounted(loadData);
</script>

<template>
  <div v-if="loading" class="flex items-center justify-center h-64">
    <p class="text-subtle">Loading dashboard...</p>
  </div>
  <div v-else class="w-full space-y-5">
    <div>
      <h1 class="text-[34px] font-bold text-base-content tracking-tight">Dashboard</h1>
      <p class="text-subtle text-base mt-1">Your complete financial overview at a glance</p>
    </div>

    <div class="grid grid-cols-4 gap-5">
      <StatCard label="Net Worth" :value="formatCurrency(totalBalance + portfolioValue)" value-class="text-income" desc="Accounts + investments" />
      <StatCard label="Total Income" :value="formatCurrency(totalIncome)" value-class="text-primary-400" desc="All time credits" />
      <StatCard label="Total Expenses" :value="formatCurrency(totalExpenses)" value-class="text-expense" desc="All time debits" />
      <StatCard label="Accounts" :value="String(accountList.length)" value-class="text-warning" desc="Connected accounts" />
    </div>

    <div class="grid grid-cols-[1fr_1.2fr] gap-5">
      <div class="card bg-base-200 border border-border">
        <div class="card-body p-6">
          <h2 class="text-lg font-semibold text-text mb-3.5">Portfolio Snapshot</h2>
          <div v-if="accountList.length > 0" class="grid grid-cols-1 gap-2.5">
            <AccountCard v-for="acc in accountList.slice(0, 4)" :key="acc.id" :account="acc" compact />
          </div>
          <p v-else class="text-center py-8 text-subtle">No accounts yet</p>
        </div>
      </div>

      <div class="card bg-base-200 border border-border">
        <div class="card-body p-6">
          <div class="flex justify-between items-center mb-2.5">
            <h2 class="text-lg font-semibold text-text">Investments Portfolio</h2>
            <router-link to="/investments" class="text-sm text-primary-400 hover:underline">View all</router-link>
          </div>
          <template v-if="portfolioByPnl.length > 0">
            <div class="grid grid-cols-[2fr_1fr_1fr] gap-3.5 pb-2 mb-1 border-b border-border text-xs text-subtle uppercase tracking-wider font-semibold">
              <div>Investment</div>
              <div class="text-right">Value</div>
              <div class="text-right">P&L</div>
            </div>
            <div
              v-for="inv in portfolioByPnl"
              :key="inv.id"
              class="grid grid-cols-[2fr_1fr_1fr] gap-3.5 py-2.5 border-b border-border text-[14.5px] last:border-0"
            >
              <div>
                <div class="text-text-secondary font-medium">{{ inv.name }}</div>
                <div class="text-[12px] text-subtle">{{ inv.symbol }}</div>
              </div>
              <div class="text-right font-medium text-text">{{ formatCurrency(inv.currentValue) }}</div>
              <div class="text-right font-bold" :class="(inv.unrealizedPnl || 0) >= 0 ? 'text-income' : 'text-expense'">
                {{ (inv.unrealizedPnl || 0) >= 0 ? '+' : '-' }}{{ formatCurrency(Math.abs(inv.unrealizedPnl || 0)) }}
              </div>
            </div>
          </template>
          <p v-else class="text-center py-8 text-subtle">No investments yet</p>
        </div>
      </div>
    </div>
  </div>
</template>
