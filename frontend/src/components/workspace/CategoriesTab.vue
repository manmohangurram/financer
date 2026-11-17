<script setup lang="ts">
import { computed, ref } from 'vue';
import { Loader2, Pencil, Trash2 } from '@lucide/vue';
import { categories } from '@/lib/api/client';
import { formatCurrency } from '@/lib/utils/format';
import AppEmpty from '@/components/ui/AppEmpty.vue';
import AppButton from '@/components/ui/AppButton.vue';

// Categories as ledger rows: name, share of spend, amount, count — and the
// recurring monthly budget, edited in place. One page owns both.
const props = defineProps<{ categories: any[]; txns: any[] }>();
const emit = defineEmits<{
  (e: 'edit', category: any): void;
  (e: 'delete', category: any): void;
  (e: 'refresh'): void;
}>();

const drafts = ref<Record<string, string>>({});
const savingId = ref('');
const rowError = ref('');

const monthStart = (() => {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
})();

const spend = computed(() => {
  const month = new Map<string, number>();
  const count = new Map<string, number>();
  for (const t of props.txns) {
    if (t.type === "CREDIT") continue;
    if (String(t.occurredAt || "").slice(0, 7) !== monthStart) continue;
    const amount = Math.abs(t.amount || 0);
    for (const id of t.categoryIds || []) {
      month.set(id, (month.get(id) || 0) + amount);
      count.set(id, (count.get(id) || 0) + 1);
    }
  }
  return { count, month };
});

const rows = computed(() =>
  [...props.categories]
    .map((c) => {
      const budget = Number(c.monthlyBudget) || 0;
      const monthSpent = spend.value.month.get(c.id) || 0;
      return {
        id: c.id,
        name: c.name,
        amount: monthSpent,
        txns: spend.value.count.get(c.id) || 0,
        budget,
        monthSpent,
        left: budget - monthSpent,
        over: budget > 0 && monthSpent > budget,
        used: budget > 0 ? Math.min(100, (monthSpent / budget) * 100) : 0
      };
    })
    .sort((a, b) => b.amount - a.amount)
);
const max = computed(() => Math.max(1, ...rows.value.map((r) => r.amount)));
const total = computed(() => rows.value.reduce((s, r) => s + r.amount, 0));
const totals = computed(() => {
  const planned = rows.value.reduce((s, r) => s + r.budget, 0);
  const spent = rows.value.reduce((s, r) => s + (r.budget > 0 ? r.monthSpent : 0), 0);
  return {
    planned,
    spent,
    left: planned - spent,
    over: spent > planned,
    used: planned > 0 ? Math.min(100, (spent / planned) * 100) : 0
  };
});

function draftFor(row: { id: string; budget: number }): string {
  if (drafts.value[row.id] !== undefined) return drafts.value[row.id];
  return row.budget > 0 ? String(row.budget) : '';
}

function setDraft(id: string, value: string) {
  drafts.value = { ...drafts.value, [id]: value };
}

async function commit(row: { id: string; budget: number }) {
  const raw = draftFor(row).trim();
  const value = raw === '' ? 0 : Number(raw);
  if (!Number.isFinite(value) || value < 0) {
    rowError.value = 'A budget must be zero or a positive amount.';
    return;
  }
  if (Math.abs(value - row.budget) < 0.005) {
    delete drafts.value[row.id];
    return;
  }
  rowError.value = '';
  savingId.value = row.id;
  try {
    await categories().setCategoryBudget({ id: row.id, monthlyBudget: value });
    delete drafts.value[row.id];
    emit('refresh');
  } catch (e: any) {
    rowError.value = e?.message || 'Could not save the budget';
  }
  savingId.value = '';
}
</script>

<template>
  <div v-if="rowError" role="alert" class="border-b border-debit/30 bg-debit/5 px-4 py-2 text-[12px] text-debit">{{ rowError }}</div>

  <div v-if="!rows.length" class="px-4 py-10">
    <AppEmpty title="No categories yet." hint="Add one to organise your spending and give it a monthly budget." />
  </div>

  <div v-else class="flex flex-col">
    <!-- the month's plan: only meaningful once a limit exists -->
    <div v-if="totals.planned > 0" class="flex flex-wrap items-center gap-x-8 gap-y-2 border-b border-rule bg-page-sunk px-4 py-2.5">
      <div>
        <p class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Planned</p>
        <p class="tabular text-[15px] font-medium text-ink">{{ formatCurrency(totals.planned) }}</p>
      </div>
      <div>
        <p class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Spent</p>
        <p class="tabular text-[15px] font-medium text-ink">{{ formatCurrency(totals.spent) }}</p>
      </div>
      <div>
        <p class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Left</p>
        <p class="tabular text-[15px] font-medium" :class="totals.over ? 'text-debit' : 'text-credit'">
          {{ totals.over ? '−' : '' }}{{ formatCurrency(Math.abs(totals.left)) }}
        </p>
      </div>
      <div class="min-w-[120px] flex-1 sm:max-w-[220px]">
        <p class="mb-1 text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Used</p>
        <div class="h-1.5 overflow-hidden rounded-full bg-page">
          <div class="h-full rounded-full" :class="totals.over ? 'bg-debit' : 'bg-accent'" :style="{ width: `${Math.max(totals.used, 2)}%` }"></div>
        </div>
      </div>
      <span class="text-[11px] text-ink-faint">monthly · repeats · debit only</span>
    </div>

    <div class="flex items-center gap-3 border-b border-rule px-4 py-2 text-label font-medium uppercase tracking-[0.06em] text-ink-mute">
      <span class="flex-1">Category</span>
      <span class="hidden w-[22%] md:block">Share this month</span>
      <span class="w-[88px] text-right">Spent</span>
      <span class="hidden w-[46px] text-right sm:block">Entries</span>
      <span class="w-[132px] text-right">Monthly budget</span>
      <span class="w-[104px]">Used</span>
      <span class="w-[96px] text-right">Left</span>
      <span class="w-[64px]"></span>
    </div>

    <div
      v-for="row in rows"
      :key="row.id"
      class="group flex items-center gap-3 border-b border-rule-soft px-4 py-[9px] last:border-0"
      :class="row.over ? 'bg-debit/5' : 'hover:bg-page-sunk/60'"
    >
      <span class="flex min-w-0 flex-1 items-center gap-1.5 text-[13px] text-ink">
        <span class="truncate">{{ row.name }}</span>
        <span v-if="row.over" class="badge badge-xs badge-outline badge-error">over</span>
      </span>

      <span class="relative hidden h-[8px] w-[22%] overflow-hidden rounded-[2px] bg-page-sunk md:block">
        <span class="block h-full border-l-2 border-accent bg-accent/25" :style="{ width: `${Math.max(2, (row.amount / max) * 100)}%` }" />
      </span>

      <span class="tabular w-[88px] text-right text-[12.5px] font-medium text-ink">{{ formatCurrency(row.amount) }}</span>
      <span class="tabular hidden w-[46px] text-right text-[12px] text-ink-mute sm:block">{{ row.txns }}</span>

      <span class="w-[132px]">
        <label class="sr-only" :for="`budget-${row.id}`">Monthly budget for {{ row.name }}</label>
        <input
          :id="`budget-${row.id}`"
          :value="draftFor(row)"
          type="number"
          min="0"
          step="0.01"
          placeholder="No budget"
          class="input input-sm input-bordered tabular w-full text-right text-[12.5px]"
          :disabled="savingId === row.id"
          @input="setDraft(row.id, ($event.target as HTMLInputElement).value)"
          @blur="commit(row)"
          @keydown.enter.prevent="commit(row)"
        />
      </span>

      <span class="w-[104px]">
        <span v-if="row.budget > 0" class="block h-1.5 overflow-hidden rounded-full bg-page-sunk" :title="`${Math.round(row.used)}% used`">
          <span class="block h-full rounded-full" :class="row.over ? 'bg-debit' : 'bg-accent'" :style="{ width: `${Math.max(row.used, 2)}%` }"></span>
        </span>
        <span v-else class="text-[11px] text-ink-faint">unbudgeted</span>
      </span>

      <span class="tabular w-[96px] text-right text-[12.5px]" :class="row.budget > 0 ? (row.over ? 'text-debit' : 'text-credit') : 'text-ink-faint'">
        <template v-if="row.budget > 0">
          {{ row.over ? '−' : '' }}{{ formatCurrency(Math.abs(row.left)) }}
        </template>
        <template v-else>—</template>
      </span>

      <span class="flex w-[64px] shrink-0 items-center justify-end gap-0.5 text-ink-mute">
        <Loader2 v-if="savingId === row.id" class="h-[14px] w-[14px] animate-spin" aria-hidden="true" />
        <AppButton variant="icon" size="sm" label="Edit category" @click="emit('edit', row)">
          <Pencil class="h-[14px] w-[14px]" stroke-width="1.7" />
        </AppButton>
        <AppButton variant="icon" size="sm" label="Delete category" @click="emit('delete', row)">
          <Trash2 class="h-[14px] w-[14px]" stroke-width="1.7" />
        </AppButton>
      </span>
    </div>

    <div class="flex items-center gap-3 border-t border-rule bg-page-sunk px-4 py-2 text-[12px] text-ink-mute">
      <span class="flex-1">{{ rows.length }} categories</span>
      <span class="hidden w-[22%] md:block"></span>
      <span class="w-[88px] text-right font-display text-[14px] text-ink">{{ formatCurrency(total) }}</span>
      <span class="hidden w-[46px] text-right sm:block">{{ rows.reduce((s, r) => s + r.txns, 0) }}</span>
      <span class="tabular w-[132px] text-right">{{ totals.planned > 0 ? formatCurrency(totals.planned) : 'no budgets' }}</span>
      <span class="w-[104px]"></span>
      <span class="w-[96px]"></span>
      <span class="w-[64px]"></span>
    </div>
  </div>
</template>
