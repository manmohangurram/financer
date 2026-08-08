<script setup lang="ts">
import { ref } from 'vue';
import Popover from '@/components/Popover.vue';
import AppInput from '@/components/AppInput.vue';
import AppSelect from '@/components/AppSelect.vue';
import DatePicker from '@/components/DatePicker.vue';
import { emptyFilters, type TransactionFilters } from '@/lib/utils/transactionFilters';

const props = defineProps<{ filters: TransactionFilters; categories: any[] }>();
const emit = defineEmits<{ (e: 'apply', filters: TransactionFilters): void; (e: 'close'): void }>();

const draft = ref<TransactionFilters>({ ...props.filters });

function clear() {
  draft.value = { ...emptyFilters(), names: props.filters.names };
}
function apply() {
  emit('apply', { ...draft.value });
}
</script>

<template>
  <Popover panel-class="right-0 w-[29rem]" no-clip @close="emit('close')">
    <div class="p-4 space-y-4">
      <h3 class="text-sm font-semibold text-text">Filter transactions</h3>

      <div class="grid grid-cols-4 gap-3">
        <div>
          <label class="block text-[12px] text-text-muted mb-1.5">From</label>
          <DatePicker v-model="draft.dateFrom" placeholder="Start" />
        </div>
        <div>
          <label class="block text-[12px] text-text-muted mb-1.5">To</label>
          <DatePicker v-model="draft.dateTo" placeholder="End" />
        </div>
        <AppInput v-model="draft.minAmount" label="Amount Min" type="number" step="0.01" placeholder="0" />
        <AppInput v-model="draft.maxAmount" label="Amount Max" type="number" step="0.01" placeholder="0" />
      </div>

      <div class="grid grid-cols-2 gap-3">
        <AppSelect v-model="draft.categoryId" label="Category">
          <option value="">All categories</option>
          <option v-for="c in categories" :key="c.id" :value="c.id">{{ c.name }}</option>
        </AppSelect>

        <AppSelect v-model="draft.type" label="Type">
          <option value="">Both</option>
          <option value="CREDIT">Credit</option>
          <option value="DEBIT">Debit</option>
        </AppSelect>
      </div>

      <div class="flex items-center justify-between pt-2">
        <button type="button" class="btn btn-ghost btn-sm text-text-muted" @click="clear">Clear</button>
        <button type="button" class="btn btn-primary btn-sm" @click="apply">Apply filters</button>
      </div>
    </div>
  </Popover>
</template>
