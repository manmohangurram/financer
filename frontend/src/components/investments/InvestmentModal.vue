<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppInput from '@/components/AppInput.vue';
import { investments } from '@/lib/api/client';
import { roundMoney } from '@/lib/utils/money';
import { guessInvestmentMapping, type InvestmentField } from '@/lib/utils/investmentImport';
import { Upload, PenLine, FileSpreadsheet } from '@lucide/vue';

const props = withDefaults(defineProps<{ investment?: any | null }>(), { investment: null });
const emit = defineEmits<{ (e: 'close'): void; (e: 'submit', payload: any): void; (e: 'delete'): void; (e: 'imported', result: { created: number; skipped: number }): void }>();

const isStock = computed(() => form.value.investmentType === 'STOCK');

const form = ref({
  name: props.investment?.name || '',
  investmentType: props.investment?.investmentType || 'STOCK',
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
    symbol: form.value.investmentType === 'MUTUAL_FUND' && !form.value.symbol ? '' : form.value.symbol,
    investmentType: form.value.investmentType,
    manualNav: form.value.investmentType === 'MUTUAL_FUND' ? roundMoney(Number(form.value.manualNav) || 0) : 0
  });
}

// --- Import mode ---
type Mode = 'manual' | 'import';
const mode = ref<Mode>('manual');
const step = ref(1);
const importError = ref('');
const importing = ref(false);
const dragOver = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);
const headers = ref<string[]>([]);
const rowCount = ref(0);
const importId = ref('');
const fieldToCol = ref<Record<InvestmentField, number>>({ symbol: -1, name: -1, type: -1, side: -1, quantity: -1, price: -1, date: -1, ignore: -1 });

const importFields: { key: InvestmentField; label: string }[] = [
  { key: 'symbol', label: 'Symbol' },
  { key: 'name', label: 'Name' },
  { key: 'type', label: 'Type (Stock/Fund)' },
  { key: 'side', label: 'Side (Buy/Sell)' },
  { key: 'quantity', label: 'Quantity/Units' },
  { key: 'price', label: 'Price/NAV' },
  { key: 'date', label: 'Date' }
];

function guessFields() {
  for (const k of Object.keys(fieldToCol.value) as InvestmentField[]) fieldToCol.value[k] = -1;
  headers.value.forEach((h, i) => {
    const g = guessInvestmentMapping(h);
    if (g !== 'ignore' && fieldToCol.value[g] === -1) fieldToCol.value[g] = i;
  });
}

function onFieldChange(field: InvestmentField, value: string) {
  fieldToCol.value[field] = parseInt(value, 10);
}

function switchMode(m: Mode) {
  mode.value = m;
  importError.value = '';
}

async function processFile(file: File | undefined | null) {
  if (!file) return;
  importError.value = '';
  try {
    const parsed = await investments().importFile(file);
    if (!parsed.headers.length) { importError.value = "Couldn't find an investments table in that file."; return; }
    importId.value = parsed.id;
    headers.value = parsed.headers;
    rowCount.value = parsed.rowCount;
    guessFields();
    step.value = 2;
  } catch (err: any) {
    importError.value = err?.message || 'Could not read that file.';
  }
}

function handleFile(e: Event) {
  const input = e.target as HTMLInputElement;
  processFile(input.files?.[0]);
  input.value = '';
}

function handleDrop(e: DragEvent) {
  dragOver.value = false;
  processFile(e.dataTransfer?.files?.[0]);
}

async function runImport() {
  importing.value = true;
  importError.value = '';
  try {
    const resp = await investments().commitImportFile({ id: importId.value, mapping: fieldToCol.value });
    emit('imported', { created: resp?.created ?? 0, skipped: resp?.skipped ?? 0 });
    emit('close');
  } catch (e: any) {
    importError.value = e?.message || 'Import failed';
  } finally {
    importing.value = false;
  }
}
</script>

<template>
  <AppModal :title="investment ? 'Edit Investment' : 'New Investment'" wide @close="emit('close')">
    <div class="p-6">
      <div class="flex gap-1 rounded-xl bg-surface border border-border p-1 mb-5" role="group" aria-label="Add method">
        <button
          type="button"
          class="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg text-[13px] font-medium transition-colors"
          :class="mode === 'manual' ? 'bg-primary-600 text-white' : 'text-subtle hover:text-text'"
          @click="switchMode('manual')"
        >
          <PenLine class="w-4 h-4" stroke-width="1.5" />
          Manual
        </button>
        <button
          v-if="!investment"
          type="button"
          class="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg text-[13px] font-medium transition-colors"
          :class="mode === 'import' ? 'bg-primary-600 text-white' : 'text-subtle hover:text-text'"
          @click="switchMode('import')"
        >
          <FileSpreadsheet class="w-4 h-4" stroke-width="1.5" />
          Import
        </button>
      </div>

      <form v-if="mode === 'manual'" @submit.prevent="handleSubmit" class="space-y-4">
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
            :class="form.investmentType === 'STOCK' ? 'btn-primary' : 'btn-ghost border border-track'"
            @click="onTypeChange('STOCK')"
          >
            Stock
          </button>
          <button
            type="button"
            class="join-item btn btn-sm flex-1"
            :class="form.investmentType === 'MUTUAL_FUND' ? 'btn-primary' : 'btn-ghost border border-track'"
            @click="onTypeChange('MUTUAL_FUND')"
          >
            Mutual Fund
          </button>
        </div>

        <AppInput v-if="!isStock" v-model.number="form.manualNav" label="Manual NAV (₹) — overrides live quote" type="number" step="0.01" min="0" />

        <div class="flex items-center justify-between pt-2">
          <button v-if="investment" type="button" class="btn btn-outline btn-error btn-sm" @click="emit('delete')">Delete</button>
          <span v-else></span>
          <div class="flex items-center gap-3">
            <button type="button" @click="emit('close')" class="btn btn-ghost btn-sm border border-track">Cancel</button>
            <button type="submit" class="btn btn-success btn-sm">{{ props.investment ? 'Update' : 'Create' }}</button>
          </div>
        </div>
      </form>

      <div v-else>
        <template v-if="step === 1">
          <div
            class="border-2 border-dashed rounded-xl p-8 text-center transition-colors"
            :class="dragOver ? 'border-primary-500 bg-primary-500/5' : 'border-border hover:border-primary-500/40'"
            @dragover.prevent="dragOver = true"
            @dragleave.prevent="dragOver = false"
            @drop.prevent="handleDrop"
          >
            <Upload class="w-10 h-10 mx-auto mb-3 text-faint" stroke-width="1.5" />
            <p class="text-[14px] text-text-muted mb-2">Drop a Groww/Kite CSV or Excel (.xlsx) holdings export, or click to browse</p>
            <button type="button" class="btn btn-outline btn-sm" @click="fileInput?.click()">Choose file</button>
            <input ref="fileInput" type="file" accept=".csv,.xlsx" class="hidden" @change="handleFile" />
          </div>
          <p v-if="importError" class="text-[13px] text-expense mt-2">{{ importError }}</p>
        </template>
        <template v-else>
          <p class="text-[13px] text-text-muted mb-3">Map your columns to the investment fields:</p>
          <div class="space-y-2 max-h-60 overflow-y-auto">
            <div v-for="f in importFields" :key="f.key" class="flex items-center gap-3">
              <span class="text-[13px] text-text w-40 shrink-0">{{ f.label }}</span>
              <select :value="fieldToCol[f.key]" @change="onFieldChange(f.key, ($event.target as HTMLSelectElement).value)" class="select w-full bg-surface border-border text-text text-[13px] flex-1">
                <option :value="-1">— None —</option>
                <option v-for="(header, i) in headers" :key="i" :value="i">{{ header }}</option>
              </select>
            </div>
          </div>
          <div class="flex items-center justify-between mt-4">
            <button @click="step = 1" class="text-[13px] text-subtle hover:text-text">Back</button>
            <button @click="runImport" class="btn btn-primary" :disabled="importing">
              {{ importing ? 'Importing...' : `Import ${rowCount} rows` }}
            </button>
          </div>
          <p v-if="importError" class="text-[13px] text-expense mt-2">{{ importError }}</p>
        </template>
      </div>
    </div>
  </AppModal>
</template>
