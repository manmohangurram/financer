<script setup lang="ts">
import { ref } from 'vue';
import Pagination from '@/components/Pagination.vue';
import TransactionTable from '@/components/workspace/TransactionTable.vue';
import { hasActiveFilters, emptyFilters, type TransactionFilters } from '@/lib/utils/transactionFilters';

const props = defineProps<{
  pagedTxns: any[];
  filteredCount: number;
  totalPages: number;
  categories: any[];
  allTxns?: any[];
  accounts?: any[];
}>();

const emit = defineEmits<{ (e: 'edit', txn: any): void; (e: 'bulk-delete', ids: string[]): void; (e: 'transfer', txn: any): void }>();

const filters = defineModel<TransactionFilters>('filters', { default: emptyFilters });
const page = defineModel<number>('page', { default: 0 });

const selectedIds = ref<string[]>([]);

function toggleSelect(txn: any) {
  const i = selectedIds.value.indexOf(txn.id);
  if (i >= 0) selectedIds.value.splice(i, 1);
  else selectedIds.value.push(txn.id);
}

function toggleSelectAll() {
  const all = props.pagedTxns.every((t) => selectedIds.value.includes(t.id));
  if (all) {
    const ids = new Set(props.pagedTxns.map((t) => t.id));
    selectedIds.value = selectedIds.value.filter((id) => !ids.has(id));
  } else {
    for (const t of props.pagedTxns) if (!selectedIds.value.includes(t.id)) selectedIds.value.push(t.id);
  }
}

function bulkDelete() {
  const ids = [...selectedIds.value];
  selectedIds.value = [];
  emit('bulk-delete', ids);
}
</script>

<template>
  <div>
    <div v-if="selectedIds.length > 0" class="flex items-center justify-between mb-3 px-4 py-2.5 rounded-xl bg-primary-500/10 border border-primary-500/30">
      <span class="text-[13px] text-primary-400 font-medium">{{ selectedIds.length }} selected</span>
      <div class="flex gap-2">
        <button @click="selectedIds = []" class="px-3 py-1.5 rounded-lg text-[12px] font-medium text-text-muted border border-border hover:border-primary-500/40 transition-colors">Cancel</button>
        <button @click="bulkDelete" class="px-3 py-1.5 rounded-lg text-[12px] font-medium bg-expense text-white hover:bg-expense/85 transition-colors">Delete selected</button>
      </div>
    </div>

    <TransactionTable
      :transactions="pagedTxns"
      :categories="categories"
      :selected-ids="selectedIds"
      :transfer-txns="allTxns"
      :accounts="accounts"
      :empty-text="hasActiveFilters(filters) ? 'No transactions match filters' : 'No transactions yet'"
      @toggle-select="toggleSelect"
      @toggle-select-all="toggleSelectAll"
      @edit="emit('edit', $event)"
      @transfer="emit('transfer', $event)"
    />
    <Pagination v-if="totalPages > 1" :page="page" :total-pages="totalPages" :label="`${filteredCount} transactions`" @page-change="page = $event" />
  </div>
</template>
