<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { analytics, type DashboardData } from '@/lib/api/client';
import AppPageHeader from '@/components/ui/AppPageHeader.vue';
import NetWorthCard from '@/components/dashboard/NetWorthCard.vue';
import BudgetCard from '@/components/dashboard/BudgetCard.vue';
import SpendingCard from '@/components/dashboard/SpendingCard.vue';
import RecentEntries from '@/components/dashboard/RecentEntries.vue';

const data = ref<DashboardData | null>(null);
const loading = ref(true);
const failed = ref(false);

const hasAccounts = computed(() => (data.value?.accounts.length ?? 0) > 0);

const monthLabel = computed(
  () =>
    data.value?.month.label ??
    new Date().toLocaleDateString('en-GB', {
      month: 'long',
      year: 'numeric'
    })
);

const eyebrow = computed(() => {
  const month = data.value?.month;
  if (month) {
    const end = new Date(`${month.to}T00:00:00`);
    const from = new Date(`${month.from}T00:00:00`);
    const days = Math.round((end.getTime() - from.getTime()) / 86400000) + 1;
    return `${month.label} · day ${days - month.daysLeft} of ${days}`;
  }
  return monthLabel.value;
});

async function load() {
  loading.value = true;
  failed.value = false;
  try {
    data.value = await analytics().dashboard({});
  } catch {
    failed.value = true;
  }
  loading.value = false;
}

onMounted(load);
</script>

<template>
  <div class="flex flex-none flex-col gap-4">
    <AppPageHeader :eyebrow="eyebrow" title="Dashboard" />

    <div
      v-if="failed"
      role="alert"
      class="alert alert-error flex items-center justify-between gap-3 rounded-card border border-debit/40 bg-page px-4 py-2.5 text-[12.5px] text-debit"
    >
      <span>The dashboard could not be loaded.</span>
      <button class="btn btn-ghost btn-xs text-debit" @click="load">Retry</button>
    </div>

    <div v-if="loading" class="flex flex-col gap-3.5" aria-busy="true" aria-label="Loading dashboard">
      <div class="grid gap-3.5 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <div class="card overflow-hidden rounded-card border border-rule bg-page">
          <div class="skeleton h-[268px] w-full bg-page-sunk"></div>
        </div>
        <div class="card overflow-hidden rounded-card border border-rule bg-page">
          <div class="skeleton h-[268px] w-full bg-page-sunk"></div>
        </div>
      </div>
      <div class="card overflow-hidden rounded-card border border-rule bg-page">
        <div class="skeleton h-[330px] w-full bg-page-sunk"></div>
      </div>
      <div class="card overflow-hidden rounded-card border border-rule bg-page">
        <div class="skeleton h-[220px] w-full bg-page-sunk"></div>
      </div>
    </div>

    <section v-else-if="!hasAccounts" class="card rounded-card border border-rule bg-page">
      <div class="card-body items-start gap-2 p-6">
        <h2 class="text-[15px] font-semibold text-ink">No accounts yet.</h2>
        <p class="text-[13px] text-ink-mute">Add an account to start booking entries.</p>
        <router-link to="/accounts/manage" class="btn btn-primary btn-sm mt-2">Add account</router-link>
      </div>
    </section>

    <template v-else-if="data">
      <div class="grid gap-3.5 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <NetWorthCard :data="data" />
        <BudgetCard :month="data.month" />
      </div>

      <SpendingCard :month="data.month" />

      <RecentEntries :entries="data.recent" :accounts="data.accounts" :total="data.recentTotal" />
    </template>
  </div>
</template>
