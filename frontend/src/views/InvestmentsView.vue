<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import InvestmentSummary from '@/components/investments/InvestmentSummary.vue';
import InvestmentTable from '@/components/investments/InvestmentTable.vue';
import InvestmentModal from '@/components/investments/InvestmentModal.vue';
import { investments } from '@/lib/api/client';
import { Plus, RefreshCw, Loader2, TrendingUp } from '@lucide/vue';

const router = useRouter();

const loading = ref(true);
const refreshing = ref(false);
const error = ref('');
const investmentList = ref<any[]>([]);
const summary = ref({ totalInvested: 0, totalCurrentValue: 0, totalUnrealizedPnl: 0, totalRealizedPnl: 0 });
const showInvestmentModal = ref(false);

async function loadAll() {
  loading.value = true;
  error.value = '';
  try {
    const [listResp, summaryResp] = await Promise.all([
      investments().listInvestments({}),
      investments().getPortfolioSummary({})
    ]);
    investmentList.value = listResp.investments || [];
    summary.value = {
      totalInvested: summaryResp.totalInvested || 0,
      totalCurrentValue: summaryResp.totalCurrentValue || 0,
      totalUnrealizedPnl: summaryResp.totalUnrealizedPnl || 0,
      totalRealizedPnl: summaryResp.totalRealizedPnl || 0
    };
  } catch (e: any) {
    error.value = e?.message || 'Failed to load investments';
  }
  loading.value = false;
}

function openDetail(inst: any) {
  router.push(`/investments/${inst.id}`);
}

async function refresh() {
  refreshing.value = true;
  error.value = '';
  try {
    await investments().refreshPrices({});
    await loadAll();
  } catch (e: any) {
    error.value = `Price refresh failed: ${e?.message || 'unknown error'} — using cached prices.`;
    await loadAll();
  }
  refreshing.value = false;
}

function openCreate() {
  showInvestmentModal.value = true;
}

async function handleInvestmentSubmit(payload: any) {
  try {
    await investments().createInvestment(payload);
    showInvestmentModal.value = false;
    await loadAll();
  } catch (e: any) {
    error.value = e?.message || 'Failed to save investment';
  }
}

onMounted(loadAll);
</script>

<template>
  <div class="w-full space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-[28px] font-bold text-base-content tracking-tight">Investments</h1>
        <p class="text-[14px] text-subtle mt-0.5">Stocks &amp; mutual funds · INR</p>
      </div>
      <div v-if="investmentList.length > 0" class="flex items-center gap-3">
        <button class="btn btn-ghost btn-sm border border-track" :disabled="refreshing" @click="refresh">
          <Loader2 v-if="refreshing" class="w-4 h-4 animate-spin" />
          <RefreshCw v-else class="w-4 h-4" />
          {{ refreshing ? 'Refreshing…' : 'Refresh prices' }}
        </button>
        <button class="btn btn-primary btn-sm gap-1.5" @click="openCreate">
          <Plus class="w-4 h-4" />
          Add Investment
        </button>
      </div>
    </div>

    <div v-if="error" class="rounded-xl border border-expense/30 bg-expense/10 px-4 py-3 text-[13px] text-expense">{{ error }}</div>

    <div v-if="loading" class="text-center py-16 text-subtle">Loading…</div>
    <div v-else-if="investmentList.length === 0" class="flex items-center justify-center min-h-[calc(100vh-180px)]">
      <div class="rounded-2xl border border-border bg-base-200 px-10 py-12 text-center max-w-md w-full">
        <div class="w-14 h-14 mx-auto rounded-2xl bg-primary-500/12 text-primary-400 flex items-center justify-center">
          <TrendingUp class="w-7 h-7" stroke-width="1.5" />
        </div>
        <h2 class="text-lg font-semibold text-text mt-4">Add your first investment</h2>
        <p class="text-[13px] text-subtle mt-1.5">Track stocks &amp; mutual funds in INR with live prices.</p>
        <button class="btn btn-primary btn-sm gap-1.5 mt-6" @click="openCreate">
          <Plus class="w-4 h-4" />
          Add Investment
        </button>
      </div>
    </div>
    <template v-else>
      <InvestmentSummary
        :total-invested="summary.totalInvested"
        :total-current-value="summary.totalCurrentValue"
        :total-unrealized-pnl="summary.totalUnrealizedPnl"
        :total-realized-pnl="summary.totalRealizedPnl"
      />

      <InvestmentTable :investments="investmentList" @select="openDetail" />
    </template>

    <InvestmentModal v-if="showInvestmentModal" @close="showInvestmentModal = false" @submit="handleInvestmentSubmit" />
  </div>
</template>
