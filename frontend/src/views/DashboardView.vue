<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { analytics, transactions } from '@/lib/api/client';
import { formatCurrency } from '@/lib/utils/format';
import AppFolio from '@/components/ui/AppFolio.vue';
import AppTable from '@/components/ui/AppTable.vue';
import AppEmpty from '@/components/ui/AppEmpty.vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppStamp from '@/components/ui/AppStamp.vue';
import AppTick from '@/components/ui/AppTick.vue';

const data = ref({
  totalBalance: 0,
  totalIncome: 0,
  totalExpenses: 0,
  portfolioValue: 0,
  accounts: [] as any[],
  investments: [] as any[]
});
const recent = ref<any[]>([]);
const loading = ref(true);
const failed = ref(false);

const netWorth = computed(() => data.value.totalBalance + data.value.portfolioValue);
const unrealised = computed(() =>
  data.value.investments.reduce((sum: number, i: any) => sum + (i.unrealizedPnl || 0), 0)
);
const accountList = computed(() => data.value.accounts);
const portfolioByPnl = computed(() =>
  [...data.value.investments].sort((a: any, b: any) => (b.unrealizedPnl || 0) - (a.unrealizedPnl || 0))
);
const signed = (t: any) => (t.type === 'DEBIT' ? -Math.abs(t.amount || 0) : Math.abs(t.amount || 0));
const pageSum = computed(() => recent.value.reduce((s: number, t: any) => s + signed(t), 0));

function accountName(id: string): string {
  const acc = accountList.value.find((a: any) => a.id === id);
  return acc ? acc.nickname || acc.bankName : '—';
}
const shortDate = (iso: string) => (iso || '').slice(0, 10);

async function load() {
  loading.value = true;
  failed.value = false;
  try {
    const [resp, txns] = await Promise.all([
      analytics().dashboard({}),
      transactions().listTransactions({ pageSize: 6 })
    ]);
    data.value = {
      totalBalance: resp.totalBalance || 0,
      totalIncome: resp.totalIncome || 0,
      totalExpenses: resp.totalExpenses || 0,
      portfolioValue: resp.portfolioValue || 0,
      accounts: resp.accounts || [],
      investments: resp.investments || []
    };
    recent.value = txns.transactions || [];
  } catch {
    failed.value = true;
  }
  loading.value = false;
}

onMounted(load);
</script>

<template>
  <div class="flex flex-col gap-5">
    <header class="flex items-end justify-between gap-4">
      <div>
        <h1 class="text-[26px] leading-tight text-ink">Dashboard</h1>
        <p class="mt-1 text-[13px] text-ink-mute">What the books say today.</p>
      </div>
      <router-link to="/accounts/spending" class="text-[13px] font-medium text-accent hover:text-accent-hover">
        Open the ledger →
      </router-link>
    </header>

    <p v-if="failed" class="flex items-center justify-between rounded-card border border-debit/40 px-4 py-2.5 text-[13px] text-debit">
      The dashboard could not be loaded.
      <AppButton variant="quiet" size="sm" @click="load">Retry</AppButton>
    </p>

    <!-- the carried-forward band: one figure, then the three that explain it -->
    <section class="flex flex-col divide-y divide-rule rounded-card border border-rule bg-page md:flex-row md:divide-x md:divide-y-0">
      <div class="min-w-[240px] px-4 py-3.5 md:px-5">
        <p class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Net worth</p>
        <p v-if="loading" class="mt-2 h-8 w-40 animate-pulse rounded-full bg-page-sunk"></p>
        <p v-else class="tabular mt-1 font-display text-[34px] leading-none tracking-[-0.02em] text-ink">
          {{ formatCurrency(netWorth) }}
        </p>
      </div>
      <div class="flex-1 px-4 py-3.5 md:px-5">
        <p class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Income</p>
        <p class="tabular mt-1 text-[16px] font-medium text-credit">+ {{ formatCurrency(data.totalIncome) }}</p>
      </div>
      <div class="flex-1 px-4 py-3.5 md:px-5">
        <p class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Expenses</p>
        <p class="tabular mt-1 text-[16px] font-medium text-debit">− {{ formatCurrency(data.totalExpenses) }}</p>
      </div>
      <div class="flex-1 px-4 py-3.5 md:px-5">
        <p class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Unrealised</p>
        <p class="tabular mt-1 text-[16px] font-medium" :class="unrealised >= 0 ? 'text-ink' : 'text-debit'">
          {{ unrealised >= 0 ? '+' : '−' }} {{ formatCurrency(Math.abs(unrealised)) }}
        </p>
      </div>
    </section>

    <div class="grid items-start gap-4 lg:grid-cols-[1.15fr_1fr]">
      <AppFolio title="Accounts" :subtitle="`${accountList.length} linked`">
        <div v-if="loading" class="flex flex-col gap-3">
          <span v-for="i in 3" :key="i" class="block h-9 animate-pulse rounded-control bg-page-sunk" />
        </div>
        <div v-else-if="accountList.length" class="flex flex-col">
          <router-link
            v-for="acc in accountList"
            :key="acc.id"
            to="/accounts"
            class="-mx-2 grid grid-cols-[32px_1fr_auto] items-center gap-2.5 rounded-control px-2 py-1.5 transition-colors hover:bg-page-sunk/70"
          >
            <span class="flex h-7 w-7 items-center justify-center rounded-[4px] border border-rule bg-page-sunk text-[10px] font-semibold uppercase text-ink-soft">
              {{ (acc.nickname || acc.bankName || '?').slice(0, 2) }}
            </span>
            <span class="min-w-0">
              <span class="flex items-center gap-1.5">
                <span class="truncate text-[13px] font-medium text-ink">{{ acc.nickname || acc.bankName }}</span>
                <AppStamp v-if="acc.endingNumbers">···{{ acc.endingNumbers }}</AppStamp>
              </span>
              <span class="block truncate text-[11px] text-ink-mute">{{ acc.bankName }}</span>
            </span>
            <span class="tabular text-[13px] font-medium" :class="(acc.balance || 0) < 0 ? 'text-debit' : 'text-ink'">
              {{ formatCurrency(acc.balance || 0) }}
            </span>
          </router-link>
        </div>
        <AppEmpty v-else title="No accounts yet." hint="Add one to start booking entries." />
      </AppFolio>

      <AppFolio title="Portfolio">
        <template #actions>
          <router-link to="/investments" class="text-[12px] text-accent hover:text-accent-hover">View all</router-link>
        </template>
        <div v-if="loading" class="flex flex-col gap-3">
          <span v-for="i in 3" :key="i" class="block h-9 animate-pulse rounded-control bg-page-sunk" />
        </div>
        <div v-else-if="portfolioByPnl.length" class="flex flex-col">
          <router-link
            v-for="inv in portfolioByPnl"
            :key="inv.id"
            :to="`/investments/${inv.id}`"
            class="-mx-2 grid grid-cols-[1fr_auto_auto] items-center gap-3 rounded-control px-2 py-1.5 transition-colors hover:bg-page-sunk/70"
          >
            <span class="min-w-0">
              <span class="block truncate text-[13px] font-medium text-ink">{{ inv.name }}</span>
              <span class="font-mono text-[11px] uppercase text-ink-mute">{{ inv.symbol }}</span>
            </span>
            <span class="tabular text-[13px] text-ink">{{ formatCurrency(inv.currentValue || 0) }}</span>
            <span class="tabular w-[74px] text-right text-[13px] font-medium" :class="(inv.unrealizedPnl || 0) >= 0 ? 'text-credit' : 'text-debit'">
              {{ (inv.unrealizedPnl || 0) >= 0 ? '+' : '−' }}{{ formatCurrency(Math.abs(inv.unrealizedPnl || 0)) }}
            </span>
          </router-link>
        </div>
        <AppEmpty v-else title="No investments yet." hint="Add an instrument to track it here." />
      </AppFolio>
    </div>

    <AppFolio title="Recent entries" flush>
      <template #actions>
        <router-link to="/accounts/spending" class="text-[12px] text-accent hover:text-accent-hover">All entries</router-link>
      </template>
      <AppTable
        :columns="[
          { label: 'Entry' },
          { label: 'Account', hideOnMobile: true },
          { label: 'Amount', align: 'right', width: '140px' }
        ]"
        :loading="loading"
        :empty="!recent.length"
      >
        <tr v-for="t in recent" :key="t.id" class="group">
          <td class="m-cell">
            <AppTick v-if="t.linkedTransferId" state="done" label="Matched transfer" class="mx-auto" />
          </td>
          <td class="border-b border-rule-soft px-3 py-2.5">
            <span class="block truncate text-[13px] text-ink">{{ t.name }}</span>
            <span class="block text-[11px] text-ink-mute">{{ shortDate(t.occurredAt) }}</span>
          </td>
          <td class="hidden border-b border-rule-soft px-3 py-2.5 text-[12px] text-ink-soft md:table-cell">{{ accountName(t.accountId) }}</td>
          <td class="tabular border-b border-rule-soft px-3 py-2.5 text-right text-[13px] font-medium" :class="(t.amount || 0) < 0 || t.type === 'DEBIT' ? 'text-debit' : 'text-credit'">
            {{ t.type === 'DEBIT' ? '−' : '+' }}{{ formatCurrency(Math.abs(t.amount || 0)) }}
          </td>
        </tr>
        <template #empty>
          <AppEmpty title="Nothing booked yet." hint="Add your first entry or import a statement.">
            <template #action>
              <router-link to="/accounts/spending"><AppButton size="sm">Add entry</AppButton></router-link>
            </template>
          </AppEmpty>
        </template>
        <template v-if="recent.length" #footer>
          <tr>
            <td class="m-cell"></td>
            <td class="bg-page-sunk px-3 py-2.5 text-[12px] text-ink-mute" colspan="2">
              Carried forward · {{ recent.length }} of the latest entries
            </td>
            <td class="tabular bg-page-sunk px-3 py-2.5 text-right font-display text-[15px] text-ink">{{ formatCurrency(pageSum) }}</td>
          </tr>
        </template>
      </AppTable>
    </AppFolio>
  </div>
</template>
