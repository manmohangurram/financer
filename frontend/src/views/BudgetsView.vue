<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { categories, analytics } from '@/lib/api/client';
import { formatCurrency } from '@/lib/utils/format';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import AppFolio from '@/components/ui/AppFolio.vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppEmpty from '@/components/ui/AppEmpty.vue';
import { Loader2, Wallet } from '@lucide/vue';

interface Category {
  id: string;
  name: string;
  monthlyBudget: number;
}

const list = ref<Category[]>([]);
const spentById = ref<Record<string, number>>({});
const loading = ref(true);
const error = ref('');
const savingId = ref('');
const rowError = ref('');
const drafts = ref<Record<string, string>>({});

const now = new Date();
const monthLabel = now.toLocaleDateString(undefined, { month: 'long', year: 'numeric' });
const monthRange = {
  dateFrom: new Date(now.getFullYear(), now.getMonth(), 1).toISOString().slice(0, 10),
  dateTo: new Date(now.getFullYear(), now.getMonth() + 1, 0).toISOString().slice(0, 10)
};

const rows = computed(() =>
  list.value.map((c) => {
    const spent = spentById.value[c.id] || 0;
    const budget = c.monthlyBudget || 0;
    const left = budget > 0 ? budget - spent : 0;
    const used = budget > 0 ? Math.min(100, (spent / budget) * 100) : 0;
    return { ...c, spent, left, used, over: budget > 0 && spent > budget };
  })
);

const totals = computed(() => {
  const planned = rows.value.reduce((s, r) => s + (r.monthlyBudget || 0), 0);
  const spent = rows.value.reduce((s, r) => s + (r.monthlyBudget > 0 ? r.spent : 0), 0);
  const left = planned - spent;
  return { planned, spent, left, over: planned > 0 && spent > planned, used: planned > 0 ? Math.min(100, (spent / planned) * 100) : 0 };
});

function draftFor(c: Category): string {
  if (drafts.value[c.id] !== undefined) return drafts.value[c.id];
  return c.monthlyBudget > 0 ? String(c.monthlyBudget) : '';
}

function setDraft(id: string, value: string) {
  drafts.value = { ...drafts.value, [id]: value };
}

async function commit(c: Category) {
  const raw = draftFor(c).trim();
  const value = raw === '' ? 0 : Number(raw);
  if (!Number.isFinite(value) || value < 0) {
    rowError.value = 'A budget must be zero or a positive amount.';
    return;
  }
  if (Math.abs(value - (c.monthlyBudget || 0)) < 0.005) {
    delete drafts.value[c.id];
    return;
  }
  rowError.value = '';
  savingId.value = c.id;
  try {
    const saved = await categories().setCategoryBudget({ id: c.id, monthlyBudget: value });
    list.value = list.value.map((x) => (x.id === c.id ? { ...x, monthlyBudget: saved.monthlyBudget } : x));
    delete drafts.value[c.id];
  } catch (e: any) {
    rowError.value = e?.message || 'Could not save the budget';
  }
  savingId.value = '';
}

async function load() {
  loading.value = true;
  error.value = '';
  try {
    const [catRes, spend] = await Promise.all([
      categories().listCategories({ pageSize: 1000 }),
      analytics().spending(monthRange)
    ]);
    list.value = (catRes.categories || []).map((c: any) => ({
      id: c.id,
      name: c.name,
      monthlyBudget: c.monthlyBudget || 0
    }));
    const byId: Record<string, number> = {};
    for (const c of spend.categories || []) byId[c.id] = c.debit || 0;
    spentById.value = byId;
  } catch (e: any) {
    error.value = e?.message || 'Failed to load budgets';
  }
  loading.value = false;
}

onMounted(load);
</script>

<template>
  <div class="flex w-full flex-col gap-4">
    <SectionHeader title="Budgets" subtitle="One amount per category, repeated every month">
      <template #actions>
        <span class="tabular text-[12.5px] text-ink-mute">{{ monthLabel }}</span>
      </template>
    </SectionHeader>

    <p v-if="error" role="alert" class="rounded-card border border-debit/40 px-4 py-2.5 text-[13px] text-debit">{{ error }}</p>
    <p v-if="rowError" role="alert" class="rounded-card border border-debit/40 px-4 py-2.5 text-[13px] text-debit">{{ rowError }}</p>

    <div v-if="loading" class="flex items-center gap-2 py-10 text-[13px] text-ink-mute">
      <Loader2 class="h-4 w-4 animate-spin" /> Loading budgets…
    </div>

    <AppFolio v-else-if="!rows.length" flush>
      <div class="px-4 py-10">
        <AppEmpty title="No categories yet." hint="Budgets hang off categories, so create one first.">
          <template #action>
            <router-link to="/accounts/categories"><AppButton size="sm">Go to Categories</AppButton></router-link>
          </template>
        </AppEmpty>
      </div>
    </AppFolio>

    <AppFolio v-else flush>
      <div class="flex flex-wrap items-center gap-x-8 gap-y-2 border-b border-rule bg-page-sunk px-4 py-3">
        <div>
          <p class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Planned</p>
          <p class="tabular text-[17px] font-medium text-ink">{{ formatCurrency(totals.planned) }}</p>
        </div>
        <div>
          <p class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Spent</p>
          <p class="tabular text-[17px] font-medium text-ink">{{ formatCurrency(totals.spent) }}</p>
        </div>
        <div>
          <p class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Left</p>
          <p class="tabular text-[17px] font-medium" :class="totals.over ? 'text-debit' : 'text-credit'">
            {{ totals.over ? '−' : '' }}{{ formatCurrency(Math.abs(totals.left)) }}
          </p>
        </div>
        <div class="min-w-[140px] flex-1 sm:max-w-[260px]">
          <p class="mb-1 text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Used</p>
          <div class="h-1.5 overflow-hidden rounded-full bg-page">
            <div class="h-full rounded-full" :class="totals.over ? 'bg-debit' : 'bg-accent'" :style="{ width: `${Math.max(totals.used, totals.planned > 0 ? 2 : 0)}%` }"></div>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-3 border-b border-rule px-4 py-2 text-label font-medium uppercase tracking-[0.06em] text-ink-mute">
        <span class="flex-1">Category</span>
        <span class="w-[150px]">Monthly budget</span>
        <span class="w-[120px] text-right">This month</span>
        <span class="w-[120px] text-right">Left</span>
        <span class="w-[120px]">Used</span>
      </div>

      <div
        v-for="r in rows"
        :key="r.id"
        class="flex items-center gap-3 border-b border-rule-soft px-4 py-[9px] last:border-0"
        :class="r.over ? 'bg-debit/5' : ''"
      >
        <span class="flex min-w-0 flex-1 items-center gap-2 text-[13px] text-ink">
          <Wallet class="h-[14px] w-[14px] shrink-0 text-ink-faint" stroke-width="1.7" aria-hidden="true" />
          <span class="truncate">{{ r.name }}</span>
          <span v-if="r.over" class="badge badge-sm badge-outline badge-error">over</span>
        </span>

        <span class="w-[150px]">
          <label class="sr-only" :for="`budget-${r.id}`">Monthly budget for {{ r.name }}</label>
          <input
            :id="`budget-${r.id}`"
            :value="draftFor(r)"
            type="number"
            min="0"
            step="0.01"
            placeholder="No budget"
            class="input input-sm input-bordered tabular w-full text-right text-[13px]"
            :disabled="savingId === r.id"
            @input="setDraft(r.id, ($event.target as HTMLInputElement).value)"
            @blur="commit(r)"
            @keydown.enter.prevent="commit(r)"
          />
        </span>

        <span class="tabular w-[120px] text-right text-[13px]" :class="r.spent ? 'text-ink' : 'text-ink-faint'">
          {{ formatCurrency(r.spent) }}
        </span>

        <span class="tabular w-[120px] text-right text-[13px]" :class="r.monthlyBudget > 0 ? (r.over ? 'text-debit' : 'text-credit') : 'text-ink-faint'">
          <template v-if="r.monthlyBudget > 0">
            {{ r.over ? '−' : '' }}{{ formatCurrency(Math.abs(r.left)) }}
          </template>
          <template v-else>—</template>
        </span>

        <span class="w-[120px]">
          <span v-if="r.monthlyBudget > 0" class="block h-1.5 overflow-hidden rounded-full bg-page-sunk" :title="`${Math.round(r.used)}% used`">
            <span class="block h-full rounded-full" :class="r.over ? 'bg-debit' : 'bg-accent'" :style="{ width: `${Math.max(r.used, 2)}%` }"></span>
          </span>
          <span v-else class="text-[11.5px] text-ink-faint">unbudgeted</span>
        </span>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-2 border-t border-rule bg-page-sunk px-4 py-2.5 text-[12px] text-ink-mute">
        <span>{{ rows.length }} categories · budgets apply to debit spend, are not carried over, and repeat every month</span>
        <span class="tabular">{{ formatCurrency(totals.planned) }} planned</span>
      </div>
    </AppFolio>
  </div>
</template>
