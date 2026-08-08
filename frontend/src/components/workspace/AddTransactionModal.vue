<script setup lang="ts">
import { ref, computed } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppSelect from '@/components/AppSelect.vue';
import TransactionForm from '@/components/workspace/TransactionForm.vue';
import { transactions } from '@/lib/api/client';
import { dateToUnixSeconds, toLocalDateString } from '@/lib/utils/format';
import { roundMoney } from '@/lib/utils/money';
import { guessMapping, mapCsvRowsToTransactions, parseCsvText, csvFileKey, type CsvField } from '@/lib/utils/csv';
import { Upload, PenLine, FileSpreadsheet } from '@lucide/vue';

const props = defineProps<{ accounts: any[]; categories: any[]; defaultAccountId: string }>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'imported'): void }>();

type Mode = 'manual' | 'csv';
const mode = ref<Mode>('manual');

const selectCls = 'select w-full bg-surface border-border text-text text-[13px]';

const txnForm = ref({
  name: '', amount: 0, type: 0,
  accountId: props.defaultAccountId,
  occurredAt: toLocalDateString(new Date()),
  categoryIds: [] as string[]
});

const step = ref(1);
const accountId = ref(props.defaultAccountId);
const headers = ref<string[]>([]);
const rows = ref<string[][]>([]);
const fileKey = ref('');
const mapping = ref<CsvField[]>([]);
const error = ref('');
const submitting = ref(false);
const importing = ref(false);
const dragOver = ref(false);
const formatMode = ref<'single' | 'split'>('single');

type FieldKey = 'date' | 'description' | 'amount' | 'type' | 'debit' | 'credit';

const requiredFields = computed<{ key: FieldKey; label: string }[]>(() =>
  formatMode.value === 'single'
    ? [
        { key: 'date', label: 'Date' },
        { key: 'description', label: 'Description' },
        { key: 'amount', label: 'Amount' },
        { key: 'type', label: 'Type (Credit/Debit)' }
      ]
    : [
        { key: 'date', label: 'Date' },
        { key: 'description', label: 'Description' },
        { key: 'debit', label: 'Debit amount' },
        { key: 'credit', label: 'Credit amount' }
      ]
);

const fieldToCol = ref<Record<FieldKey, number>>({ date: -1, description: -1, amount: -1, type: -1, debit: -1, credit: -1 });

function buildMapping() {
  mapping.value = headers.value.map((_, i) => {
    const key = (Object.keys(fieldToCol.value) as FieldKey[]).find((k) => fieldToCol.value[k] === i);
    return key ? (key as CsvField) : 'ignore';
  });
}

function guessForMode(header: string): CsvField {
  const g = guessMapping(header);
  if (formatMode.value === 'single' && (g === 'debit' || g === 'credit')) return 'amount';
  if (formatMode.value === 'split' && (g === 'amount' || g === 'type')) return 'ignore';
  return g;
}

function guessFields() {
  for (const k of Object.keys(fieldToCol.value) as FieldKey[]) fieldToCol.value[k] = -1;
  for (let i = 0; i < headers.value.length; i++) {
    const g = guessForMode(headers.value[i]);
    if (g !== 'ignore' && fieldToCol.value[g] === -1) fieldToCol.value[g] = i;
  }
  buildMapping();
}

function switchFormat(mode: 'single' | 'split') {
  formatMode.value = mode;
  guessFields();
}

function onFieldChange(field: { key: FieldKey }, value: string) {
  fieldToCol.value[field.key] = parseInt(value, 10);
  buildMapping();
}

function switchMode(m: Mode) {
  mode.value = m;
  error.value = '';
}

function processFile(file: File | undefined | null) {
  if (!file) return;
  file.text().then((text) => {
    const parsed = parseCsvText(text);
    if (!parsed) { error.value = 'Need header + at least 1 data row.'; return; }
    headers.value = parsed.headers;
    rows.value = parsed.rows;
    fileKey.value = csvFileKey(text);
    guessFields();
    step.value = 2;
  });
}

async function handleFile(e: Event) {
  const input = e.target as HTMLInputElement;
  processFile(input.files?.[0]);
  input.value = '';
}

function handleDrop(e: DragEvent) {
  dragOver.value = false;
  processFile(e.dataTransfer?.files?.[0]);
}

async function submitManual() {
  submitting.value = true;
  error.value = '';
  try {
    const payload = { ...txnForm.value, amount: roundMoney(txnForm.value.amount), occurredAt: { seconds: dateToUnixSeconds(txnForm.value.occurredAt), nanos: 0 } };
    await transactions().createTransactions({ transactions: [payload] });
    emit('imported');
    emit('close');
  } catch (e: any) {
    error.value = e.message || 'Failed to add transaction';
  }
  submitting.value = false;
}

async function runImport() {
  importing.value = true;
  error.value = '';
  try {
    const txns = mapCsvRowsToTransactions(rows.value, mapping.value, accountId.value, fileKey.value);
    if (txns.length === 0) {
      error.value = 'No valid rows to import — check the column mapping and data.';
      return;
    }
    // One call holds at most 500 rows; larger files are split into batches.
    const BATCH = 500;
    for (let i = 0; i < txns.length; i += BATCH) {
      const batch = txns.slice(i, i + BATCH);
      await transactions().createTransactions({ transactions: batch });
    }
    emit('imported');
    emit('close');
  } catch (e: any) {
    error.value = e.message || 'Import failed';
  }
  importing.value = false;
}
</script>

<template>
  <AppModal title="Add Transaction" wide @close="emit('close')">
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
          type="button"
          class="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg text-[13px] font-medium transition-colors"
          :class="mode === 'csv' ? 'bg-primary-600 text-white' : 'text-subtle hover:text-text'"
          @click="switchMode('csv')"
        >
          <FileSpreadsheet class="w-4 h-4" stroke-width="1.5" />
          Import CSV
        </button>
      </div>

      <form v-if="mode === 'manual'" @submit.prevent="submitManual" class="space-y-4">
        <TransactionForm v-model="txnForm" :accounts="accounts" :categories="categories" />
        <p v-if="error" class="text-[13px] text-expense">{{ error }}</p>
        <div class="flex justify-end pt-2">
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            {{ submitting ? 'Adding...' : 'Add Transaction' }}
          </button>
        </div>
      </form>

      <div v-else>
        <template v-if="step === 1">
          <AppSelect v-model="accountId" label="Account">
            <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.accountNickname || a.bankName }}</option>
          </AppSelect>
          <div
            class="mt-4 border-2 border-dashed rounded-xl p-8 text-center transition-colors"
            :class="dragOver ? 'border-primary-500 bg-primary-500/5' : 'border-border hover:border-primary-500/40'"
            @dragover.prevent="dragOver = true"
            @dragleave.prevent="dragOver = false"
            @drop.prevent="handleDrop"
          >
            <Upload class="w-10 h-10 mx-auto mb-3 text-faint" stroke-width="1.5" />
            <p class="text-[14px] text-text-muted mb-2">Drop a CSV file or click to browse</p>
            <label class="cursor-pointer text-[13px] text-primary-400 hover:underline">
              Choose file
              <input type="file" accept=".csv" class="hidden" @change="handleFile" />
            </label>
          </div>
          <p v-if="error" class="text-[13px] text-expense mt-2">{{ error }}</p>
        </template>
        <template v-else>
          <div class="flex gap-1 rounded-xl bg-surface border border-border p-1 mb-4" role="group" aria-label="CSV format">
            <button
              type="button"
              class="flex-1 px-3 py-1.5 rounded-lg text-[12px] font-medium transition-colors"
              :class="formatMode === 'single' ? 'bg-primary-600 text-white' : 'text-subtle hover:text-text'"
              @click="switchFormat('single')"
            >Single amount column</button>
            <button
              type="button"
              class="flex-1 px-3 py-1.5 rounded-lg text-[12px] font-medium transition-colors"
              :class="formatMode === 'split' ? 'bg-primary-600 text-white' : 'text-subtle hover:text-text'"
              @click="switchFormat('split')"
            >Debit + Credit columns</button>
          </div>
          <p class="text-[13px] text-text-muted mb-3">Map your columns to the CSV fields:</p>
          <div class="space-y-2 max-h-60 overflow-y-auto">
            <div v-for="f in requiredFields" :key="f.key" class="flex items-center gap-3">
              <span class="text-[13px] text-text w-36 shrink-0">{{ f.label }}</span>
              <select :value="fieldToCol[f.key]" @change="onFieldChange(f, ($event.target as HTMLSelectElement).value)" :class="[selectCls, 'flex-1']">
                <option :value="-1">— None —</option>
                <option v-for="(header, i) in headers" :key="i" :value="i">{{ header }}</option>
              </select>
            </div>
          </div>
          <div class="flex items-center justify-between mt-4">
            <button @click="step = 1" class="text-[13px] text-subtle hover:text-text">Back</button>
            <button @click="runImport" class="btn btn-primary" :disabled="importing">
              {{ importing ? 'Importing...' : `Import ${rows.length} rows` }}
            </button>
          </div>
          <p v-if="error" class="text-[13px] text-expense mt-2">{{ error }}</p>
        </template>
      </div>
    </div>
  </AppModal>
</template>