<script setup lang="ts">
import { computed } from 'vue';
import type { DashboardAccount, DashboardTransaction } from '@/lib/api/client';
import { formatAmount } from '@/lib/utils/format';
import AppTable, { type Column } from '@/components/ui/AppTable.vue';

const props = defineProps<{
  entries: DashboardTransaction[];
  accounts: DashboardAccount[];
  total: number;
}>();

const money = (value: number) => formatAmount(Math.abs(Math.round(value)));

const columns: Column[] = [{ label: 'Date', width: '96px' }, { label: 'Entry' }, { label: 'Amount', align: 'right' }];

const accountNames = computed(() => {
  const map = new Map<string, string>();
  for (const account of props.accounts) map.set(account.id, account.nickname || account.bankName || 'Account');
  return map;
});

function dateLabel(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso.slice(0, 10);
  return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
}
</script>

<template>
  <section class="card overflow-hidden rounded-card border border-rule bg-page" aria-labelledby="recent-title">
    <header class="flex items-start justify-between gap-4 border-b border-rule px-4 pb-3 pt-3.5 md:px-5">
      <div class="min-w-0">
        <h2 id="recent-title" class="text-[14px] font-semibold text-ink">Recent entries</h2>
        <p class="mt-1 text-[11.5px] text-ink-mute">{{ entries.length }} latest · newest first</p>
      </div>
      <router-link
        to="/accounts/spending"
        class="link link-hover shrink-0 pt-0.5 text-[11.5px] font-medium text-accent"
      >
        All entries →
      </router-link>
    </header>

    <AppTable :columns="columns" :margin="false" :empty="entries.length === 0" dense>
      <template #empty>
        <p class="text-[13px] text-ink">Nothing booked yet.</p>
        <router-link
          to="/accounts/spending"
          class="link link-hover mt-1 inline-block text-[12px] font-medium text-accent"
        >
          Add an entry →
        </router-link>
      </template>

      <tr v-for="entry in entries" :key="entry.id">
        <td class="border-b border-rule-soft px-3 py-2.5 align-middle md:px-4">
          <span class="num text-[11px] text-ink-mute">{{ dateLabel(entry.occurredAt) }}</span>
        </td>
        <td class="border-b border-rule-soft px-3 py-2.5 align-middle md:px-4">
          <span class="block truncate text-[12.5px] text-ink">{{ entry.name }}</span>
          <span class="mt-0.5 block text-[11px] text-ink-mute">
            {{ accountNames.get(entry.accountId) || 'Account' }}
            <template v-if="entry.linkedTransferId"> · matched</template>
          </span>
        </td>
        <td
          class="num whitespace-nowrap border-b border-rule-soft px-3 py-2.5 text-right text-[12.5px] md:px-4"
          :class="entry.type === 'DEBIT' ? 'text-debit' : 'text-credit'"
        >
          {{ entry.type === 'DEBIT' ? '−' : '+' }}{{ money(entry.amount ?? 0) }}
        </td>
      </tr>

      <template #footer>
        <tr>
          <td colspan="2" class="bg-page-sunk/50 px-4 py-2.5 text-[11px] text-ink-mute md:px-5">Carried forward</td>
          <td class="num bg-page-sunk/50 px-4 py-2.5 text-right text-[12.5px] text-ink md:px-5">
            {{ money(total) }}
          </td>
        </tr>
      </template>
    </AppTable>
  </section>
</template>
