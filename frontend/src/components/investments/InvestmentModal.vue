<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppInput from '@/components/AppInput.vue';
import { investments } from '@/lib/api/client';
import { roundMoney } from '@/lib/utils/money';

const props = withDefaults(defineProps<{ investment?: any | null }>(), { investment: null });
const emit = defineEmits<{ (e: 'close'): void; (e: 'submit', payload: any): void }>();

const isStock = computed(() => form.value.investmentType === 'INVESTMENT_TYPE_STOCK');

const form = ref({
  name: props.investment?.name || '',
  investmentType: props.investment?.investmentType || 'INVESTMENT_TYPE_STOCK',
  symbol: props.investment?.symbol || '',
  manualNav: props.investment?.manualNav || 0
});

const searching = ref(false);
const results = ref<any[]>([]);
let debounceTimer: ReturnType<typeof setTimeout> | null = null;

function onTypeChange(t: string) {
  form.value.investmentType = t;
  results.value = [];
}

watch(
  () => form.value.symbol,
  (val) => {
    if (debounceTimer) clearTimeout(debounceTimer);
    results.value = [];
    if (!val || val.trim().length < 2) return;
    searching.value = false;
    debounceTimer = setTimeout(async () => {
      searching.value = true;
      try {
        const resp = await investments().searchSymbols({ query: val.trim() });
        results.value = resp.results || [];
      } catch {
        results.value = [];
      }
      searching.value = false;
    }, 300);
  }
);

function pickResult(r: any) {
  form.value.symbol = r.symbol;
  form.value.name = r.name;
  results.value = [];
}

function handleSubmit() {
  emit('submit', {
    id: props.investment?.id,
    name: form.value.name,
    symbol: form.value.investmentType === 'INVESTMENT_TYPE_MUTUAL_FUND' && !form.value.symbol ? '' : form.value.symbol,
    investmentType: form.value.investmentType,
    manualNav: form.value.investmentType === 'INVESTMENT_TYPE_MUTUAL_FUND' ? roundMoney(Number(form.value.manualNav) || 0) : 0
  });
}
</script>

<template>
  <AppModal :title="investment ? 'Edit Investment' : 'New Investment'" @close="emit('close')">
    <form @submit.prevent="handleSubmit" class="p-6 space-y-4">
      <AppInput
        v-model="form.symbol"
        :label="isStock ? 'Symbol' : 'Symbol (optional)'"
        :placeholder="isStock ? 'e.g. RELIANCE.NS — type to search' : 'e.g. 0P0001RK6V.BO — type to search'"
        :required="isStock"
        autocomplete="off"
      >
        <template #dropdown>
          <div
            v-if="results.length || searching"
            class="absolute left-0 right-0 top-full z-50 mt-1 max-h-56 overflow-auto rounded-xl border border-border bg-base-200 shadow-card"
          >
            <div v-if="searching" class="px-4 py-3 text-[13px] text-subtle">Searching…</div>
            <button
              v-for="r in results"
              :key="r.symbol"
              type="button"
              class="w-full text-left px-4 py-3 hover:bg-white/5 transition-colors"
              @click="pickResult(r)"
            >
              <div class="font-medium text-text">{{ r.symbol }}</div>
              <div class="text-[12px] text-subtle">{{ r.name }}</div>
            </button>
            <div v-if="!searching && results.length === 0" class="px-4 py-3 text-[13px] text-subtle">No matches</div>
          </div>
        </template>
      </AppInput>

      <AppInput v-model="form.name" label="Name" placeholder="e.g. Reliance Industries" required />

      <label class="label p-0 pb-1"><span class="label-text text-text-muted">Type</span></label>
      <div class="join w-full">
        <button
          type="button"
          class="join-item btn btn-sm flex-1"
          :class="form.investmentType === 'INVESTMENT_TYPE_STOCK' ? 'btn-primary' : 'btn-ghost border border-track'"
          @click="onTypeChange('INVESTMENT_TYPE_STOCK')"
        >
          Stock
        </button>
        <button
          type="button"
          class="join-item btn btn-sm flex-1"
          :class="form.investmentType === 'INVESTMENT_TYPE_MUTUAL_FUND' ? 'btn-primary' : 'btn-ghost border border-track'"
          @click="onTypeChange('INVESTMENT_TYPE_MUTUAL_FUND')"
        >
          Mutual Fund
        </button>
      </div>

      <AppInput v-if="!isStock" v-model.number="form.manualNav" label="Manual NAV (₹) — overrides live quote" type="number" step="0.01" min="0" />

      <div class="flex items-center justify-end gap-3 pt-2">
        <button type="button" @click="emit('close')" class="btn btn-ghost btn-sm border border-track">Cancel</button>
        <button type="submit" class="btn btn-success btn-sm">{{ props.investment ? 'Update' : 'Create' }}</button>
      </div>
    </form>
  </AppModal>
</template>