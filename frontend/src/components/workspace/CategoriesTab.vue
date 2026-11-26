<script setup lang="ts">
import { computed } from 'vue';
import { formatCurrency } from '@/lib/utils/format';
import AppEmpty from '@/components/ui/AppEmpty.vue';

// Categories is a name and a monthly budget. Spending owns the analysis.
// A row opens the edit popup, which also carries Delete.
const props = defineProps<{ categories: any[] }>();
const emit = defineEmits<{ (e: 'edit', category: any): void }>();

const rows = computed(() =>
  [...props.categories].map((c) => {
    const budget = Number(c.monthlyBudget) || 0;
    return { id: c.id, name: c.name, budget, monthlyBudget: budget };
  })
);
</script>

<template>
  <div v-if="!rows.length" class="px-4 py-10">
    <AppEmpty title="No categories yet." hint="Add one to organise spending and give it a monthly budget." />
  </div>

  <div v-else class="flex flex-col">
    <div
      class="flex items-center gap-3 border-b border-rule px-4 py-2 text-label font-medium uppercase tracking-[0.1em] text-ink-mute"
    >
      <span class="flex-1">Category</span>
      <span class="w-[160px] text-right">Monthly budget</span>
    </div>

    <button
      v-for="row in rows"
      :key="row.id"
      type="button"
      class="flex items-center gap-3 border-b border-rule-soft px-4 py-[10px] text-left last:border-0 hover:bg-page-sunk/60 focus-visible:bg-page-sunk/60 focus-visible:outline-none"
      @click="emit('edit', row)"
    >
      <span class="min-w-0 flex-1 truncate text-[13px] text-ink">{{ row.name }}</span>
      <span class="num w-[160px] text-right text-[13px]" :class="row.budget > 0 ? 'text-ink' : 'text-ink-faint'">
        {{ row.budget > 0 ? formatCurrency(row.budget) : 'No budget' }}
      </span>
    </button>

    <div class="flex items-center gap-3 border-t border-rule bg-page-sunk px-4 py-2 text-[12px] text-ink-mute">
      <span class="flex-1">{{ rows.length }} categories</span>
      <span class="w-[160px]"></span>
    </div>
  </div>
</template>
