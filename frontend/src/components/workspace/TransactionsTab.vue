<script setup lang="ts">
import { ref } from 'vue';
import Pagination from '@/components/Pagination.vue';
import TransactionTable from '@/components/workspace/TransactionTable.vue';
import { buildActiveFilterChips, clearFilterKey, emptyFilters, type TransactionFilters } from '@/lib/utils/transactionFilters';

const props = defineProps<{
  pagedTxns: any[];
  filteredCount: number;
  totalPages: number;
  categories: any[];
  allTxns?: any[];
  accounts?: any[];
}>();

const emit = defineEmits<{ (e: 'edit', txn: any): void; (e: 'bulk-delete', ids: string[]): void }>();

const showCheckboxes = defineModel<boolean>('showCheckboxes', { default: false });
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

function removeFilter(key: keyof TransactionFilters) {
  filters.value = clearFilterKey(filters.value, key);
  page.value = 0;
}

function clearFilters() {
  filters.value = emptyFilters();
  page.value = 0;
}
</script>

<template>
  <div>
    <div v-if="buildActiveFilterChips(filters, categories).length > 0" class="flex flex-wrap gap-2 mb-3">      <span
        v-for="chip in buildActiveFilterChips(filters, categories)"
        :key="chip.key"
        class="badge badge-error badge-outline gap-1 text-[12px]"
      >
        {{ chip.label }}
        <button class="hover:text-base-content" @click="removeFilter(chip.key)">✕</button>
      </span>
      <button @click="clearFilters" class="text-[12px] text-subtle hover:text-text self-center ml-1">Clear all</button>
    </div>

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
      :show-checkboxes="showCheckboxes"
      :selected-ids="selectedIds"
      :transfer-txns="allTxns"
      :accounts="accounts"
      :empty-text="buildActiveFilterChips(filters, categories).length ? 'No transactions match filters' : 'No transactions yet'"
      @toggle-select="toggleSelect"
      @toggle-select-all="toggleSelectAll"
      @edit="emit('edit', $event)"
    />
    <Pagination v-if="totalPages > 1" :page="page" :total-pages="totalPages" :label="`${filteredCount} transactions`" @page-change="page = $event" />
  </div>
</template>
