<script setup lang="ts">
import { computed, ref } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppSteps from '@/components/ui/AppSteps.vue';
import { transactions } from '@/lib/api/client';
import { readStatement, type StatementParse, type StatementLayout } from '@/lib/utils/statementPdf';

const props = defineProps<{ accounts: any[]; defaultAccountId?: string }>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'imported'): void }>();

const steps = ['Upload', 'Map columns', 'Review'];
const step = ref(0);
const accountId = ref(props.defaultAccountId || props.accounts[0]?.id || '');
const pickedFile = ref<File | null>(null);
const fileName = ref('');
const parsing = ref(false);
const importing = ref(false);
const progress = ref(0);
const error = ref('');
const parsed = ref<StatementParse | null>(null);
const layout = ref<'auto' | StatementLayout>('auto');
const duplicates = ref<boolean[]>([]);
const importAll = ref(false);
const checking = ref(false);

const canContinue = computed(() => {
  if (step.value === 0) return !!pickedFile.value && !parsing.value;
  if (step.value === 1) return !!parsed.value?.valid;
  if (step.value === 2) return !!parsed.value?.valid && !!accountId.value && !importing.value && !checking.value && rowsToImport.value > 0;
  return !!parsed.value?.valid && !!accountId.value && !importing.value;
});

const detectedLabel = computed(() => {
  switch (parsed.value?.layout) {
    case 'columns':
      return 'Header columns � amount columns from the statement';
    case 'withdrawal-deposit':
      return 'Withdrawal · Deposit · Balance';
    case 'amount-marker':
      return 'Amount · Dr/Cr · Balance';
    case 'amount-delta':
      return 'Amount · Balance (direction from balance)';
    default:
      return 'Amount · Dr/Cr (no balance)';
  }
});

async function parsePicked() {
  if (!pickedFile.value) return;
  error.value = '';
  parsing.value = true;
  try {
    const result = await readStatement(pickedFile.value, layout.value === 'auto' ? undefined : layout.value);
    parsed.value = result;
    if (!result.valid) error.value = result.problem || 'Could not validate this statement.';
  } catch (e: any) {
    error.value = e?.message || 'Could not read this PDF.';
    parsed.value = null;
  }
  parsing.value = false;
}

async function onFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const next = input.files?.[0];
  if (!next) return;
  pickedFile.value = next;
  fileName.value = next.name;
  parsed.value = null;
  layout.value = 'auto';
  step.value = 0;
  await parsePicked();
  input.value = '';
}

async function changeLayout() {
  duplicates.value = [];
  importAll.value = false;
  if (pickedFile.value) await parsePicked();
}

function goTo(index: number) {
  if (index <= step.value) step.value = index;
}

function back() {
  error.value = '';
  if (step.value > 0) step.value -= 1;
}

async function checkDuplicates() {
  if (!parsed.value || parsed.value.rows.length === 0) return;
  checking.value = true;
  error.value = '';
  try {
    const flags: boolean[] = [];
    const rows = parsed.value.rows;
    for (let i = 0; i < rows.length; i += 500) {
      const slice = rows.slice(i, i + 500).map((r) => ({
        name: r.name,
        amount: r.amount,
        type: r.type,
        occurredAt: r.occurredAt,
        refNo: r.refNo || undefined,
        accountId: accountId.value
      }));
      const out = await transactions().findDuplicates({ transactions: slice });
      flags.push(...(out.duplicates || []));
    }
    duplicates.value = flags;
  } catch (e: any) {
    duplicates.value = [];
    error.value = e?.message || 'Could not check for duplicates.';
  }
  checking.value = false;
}

async function next() {
  error.value = '';
  if (step.value === 0) {
    if (!parsed.value) {
      error.value = 'Choose a PDF first.';
      return;
    }
    step.value = 1;
    return;
  }
  if (step.value === 1) {
    if (!parsed.value?.valid) return;
    step.value = 2;
    await checkDuplicates();
  }
}

const duplicateCount = computed(() => duplicates.value.filter(Boolean).length);
const duplicateSamples = computed(() =>
  parsed.value ? parsed.value.rows.filter((_r, i) => duplicates.value[i]).slice(0, 3) : []
);
const rowsToImport = computed(() => {
  if (!parsed.value) return 0;
  return importAll.value ? parsed.value.rows.length : parsed.value.rows.length - duplicateCount.value;
});

async function importRows() {
  if (!parsed.value || !canContinue.value) return;
  importing.value = true;
  error.value = '';
  progress.value = 0;
  const rows = importAll.value
    ? parsed.value.rows
    : parsed.value.rows.filter((_r, i) => !duplicates.value[i]);
  try {
    for (let i = 0; i < rows.length; i += 100) {
      const slice = rows.slice(i, i + 100).map((r) => ({
        id: crypto.randomUUID(),
        name: r.name,
        amount: r.amount,
        type: r.type,
        occurredAt: r.occurredAt,
        refNo: r.refNo || undefined,
        accountId: accountId.value
      }));
      await transactions().createTransactions({ transactions: slice });
      progress.value = Math.min(rows.length, i + slice.length);
    }
    emit('imported');
    emit('close');
  } catch (e: any) {
    error.value = e?.message || 'Import failed part-way; check the account and retry.';
  }
  importing.value = false;
}
</script>

<template>
  <AppModal title="Import a bank statement" roomy steady @close="emit('close')">
    <div class="flex h-full min-h-0 flex-col px-[26px] pt-[18px] pb-[14px]">
      <AppSteps :steps="steps" :current="step" @select="goTo" />

      <div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pt-4">
        <p v-if="error" role="alert" class="m-0 shrink-0 rounded-control border border-debit/30 bg-debit/5 px-3 py-2 text-[12.5px] text-debit">
          {{ error }}
        </p>
        <template v-if="step === 0">
          <p class="m-0 text-[13px] leading-relaxed text-ink-mute">
            Choose a statement PDF. Rows are read in your browser, checked against the statement's own balances, then booked.
            Nothing leaves this device until you press Import.
          </p>

          <label class="flex cursor-pointer flex-col items-center gap-3 rounded-folio border-[1.5px] border-dashed border-rule-strong bg-page-sunk px-6 py-7 text-center transition-colors hover:border-accent/60 hover:bg-accent-wash/40 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-accent/40">
            <span class="grid h-11 w-11 place-items-center rounded-folio bg-accent-wash text-[18px] text-accent" aria-hidden="true">↑</span>
            <span class="flex flex-col gap-0.5">
              <span class="text-[13.5px] font-semibold text-ink">{{ fileName || 'Drop a PDF here, or choose a file' }}</span>
              <span class="text-[12px] text-ink-mute">
                {{ parsing ? 'Reading…' : 'Bank statement · PDF only' }}
              </span>
            </span>
            <input type="file" accept="application/pdf" class="sr-only" @change="onFile" />
          </label>

          <ul class="m-0 flex list-none flex-col gap-1.5 p-0 text-[12px] text-ink-mute">
            <li class="flex items-start gap-2"><span class="mt-[6px] h-1 w-1 shrink-0 rounded-full bg-ink-faint" aria-hidden="true" />Parsed in your browser — the PDF is not uploaded.</li>
            <li class="flex items-start gap-2"><span class="mt-[6px] h-1 w-1 shrink-0 rounded-full bg-ink-faint" aria-hidden="true" />Every row is checked against the statement's own balances.</li>
            <li class="flex items-start gap-2"><span class="mt-[6px] h-1 w-1 shrink-0 rounded-full bg-ink-faint" aria-hidden="true" />Nothing is booked until you press Import on the last step.</li>
          </ul>

          <label class="flex flex-col gap-1.5">
            <span class="text-[11px] font-medium uppercase tracking-[0.06em] text-ink-mute">Book into</span>
            <select v-model="accountId" class="select select-bordered h-10 w-full text-[13.5px]">
              <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.nickname || a.bankName }}</option>
            </select>
          </label>

          <p v-if="parsed" class="m-0 flex flex-wrap items-center gap-2 text-[12.5px] text-ink-soft">
            <span>{{ fileName }}</span><span class="text-ink-faint">·</span>
            <span class="tabular">{{ parsed.rows.length.toLocaleString() }} rows</span><span class="text-ink-faint">·</span>
            <span class="tabular">{{ parsed.pages }} pages</span><span class="text-ink-faint">·</span>
            <span class="stamp" :class="parsed.valid ? 'border-credit/40 text-credit' : 'border-debit/40 text-debit'">
              {{ parsed.valid ? 'balances match' : 'balances do not match' }}
            </span>
          </p>
        </template>
        <template v-else-if="step === 1 && parsed">
          <div class="flex flex-wrap items-center gap-2 rounded-control border border-rule bg-page-sunk px-3.5 py-2.5 text-[12.5px]">
            <span class="font-mono text-ink-soft">{{ fileName }}</span>
            <span class="text-ink-faint">·</span>
            <span class="tabular">{{ parsed.rows.length.toLocaleString() }} rows</span>
            <span class="text-ink-faint">·</span>
            <span class="stamp accent">detected: {{ detectedLabel }}</span>
          </div>

          <label class="flex flex-col gap-1.5">
            <span class="text-[11px] font-medium uppercase tracking-[0.06em] text-ink-mute">Column layout</span>
            <select v-model="layout" class="select select-bordered h-10 w-full text-[13.5px]" @change="changeLayout">
              <option value="auto">Detect automatically</option>
              <option value="withdrawal-deposit">Withdrawal · Deposit · Balance</option>
              <option value="amount-marker">Amount · Dr/Cr marker · Balance</option>
              <option value="amount-delta">Amount · Balance (direction from balance)</option>
              <option value="amount-only">Amount · Dr/Cr, no balance</option>
            </select>
            <span class="text-[11.5px] text-ink-mute">
              The date is always the first date column; the balance is the last amount when the layout has one.
            </span>
          </label>

          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div class="flex flex-col gap-0.5 rounded-control border border-rule px-3 py-2">
              <span class="text-[10px] uppercase tracking-[0.06em] text-ink-faint">Date</span>
              <span class="font-mono text-[12px] text-ink">{{ parsed.rows[0]?.occurredAt || '—' }}</span>
            </div>
            <div class="flex flex-col gap-0.5 rounded-control border border-rule px-3 py-2">
              <span class="text-[10px] uppercase tracking-[0.06em] text-ink-faint">Amount</span>
              <span class="font-mono text-[12px] text-ink">{{ parsed.rows[0]?.amount?.toLocaleString() || '—' }}</span>
            </div>
            <div class="flex flex-col gap-0.5 rounded-control border border-rule px-3 py-2">
              <span class="text-[10px] uppercase tracking-[0.06em] text-ink-faint">Direction</span>
              <span class="font-mono text-[12px] text-ink">{{ parsed.rows[0]?.type || '—' }}</span>
            </div>
            <div class="flex flex-col gap-0.5 rounded-control border border-rule px-3 py-2">
              <span class="text-[10px] uppercase tracking-[0.06em] text-ink-faint">Balance</span>
              <span class="font-mono text-[12px] text-ink">{{ parsed.rows[0]?.balanceAfter?.toLocaleString() ?? '—' }}</span>
            </div>
          </div>

          <div class="overflow-hidden rounded-control border border-rule">
            <table class="w-full text-[12px]">
              <thead class="bg-page-sunk text-left text-[10.5px] uppercase tracking-[0.06em] text-ink-mute">
                <tr><th class="px-2.5 py-2">Date</th><th class="px-2.5 py-2">Name</th><th class="px-2.5 py-2">Reference</th><th class="px-2.5 py-2 text-right">Amount</th><th class="px-2.5 py-2 text-right">Balance</th></tr>
              </thead>
              <tbody>
                <tr v-for="(r, i) in parsed.rows.slice(0, 6)" :key="i" class="border-t border-rule-soft">
                  <td class="whitespace-nowrap px-2.5 py-1.5 text-ink-mute">{{ r.occurredAt }}</td>
                  <td class="max-w-[300px] truncate px-2.5 py-1.5 text-ink-soft">{{ r.name }}</td>
                  <td class="whitespace-nowrap px-2.5 py-1.5 font-mono text-[11px] text-ink-mute">{{ r.refNo || '—' }}</td>
                  <td class="tabular whitespace-nowrap px-2.5 py-1.5 text-right" :class="r.type === 'DEBIT' ? 'text-debit' : 'text-credit'">
                    {{ r.type === 'DEBIT' ? '−' : '+' }}{{ r.amount.toLocaleString() }}
                  </td>
                  <td class="tabular whitespace-nowrap px-2.5 py-1.5 text-right text-ink-soft">{{ r.balanceAfter?.toLocaleString() ?? '—' }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <p class="m-0 text-[11.5px] text-ink-mute">Showing the first 6 of {{ parsed.rows.length.toLocaleString() }} rows.</p>
        </template>
        <template v-else-if="step === 2 && parsed">
          <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <div class="flex flex-col gap-0.5"><span class="text-[10.5px] uppercase tracking-[0.06em] text-ink-mute">Rows</span><span class="tabular text-[15px] font-semibold text-ink">{{ parsed.rows.length.toLocaleString() }}</span></div>
            <div class="flex flex-col gap-0.5"><span class="text-[10.5px] uppercase tracking-[0.06em] text-ink-mute">Opening</span><span class="tabular text-[15px] font-semibold text-ink">{{ parsed.opening?.toLocaleString() ?? '—' }}</span></div>
            <div class="flex flex-col gap-0.5"><span class="text-[10.5px] uppercase tracking-[0.06em] text-ink-mute">Credits</span><span class="tabular text-[15px] font-semibold text-credit">{{ parsed.credits.toLocaleString() }}</span></div>
            <div class="flex flex-col gap-0.5"><span class="text-[10.5px] uppercase tracking-[0.06em] text-ink-mute">Debits</span><span class="tabular text-[15px] font-semibold text-debit">{{ parsed.debits.toLocaleString() }}</span></div>
            <div class="flex flex-col gap-0.5"><span class="text-[10.5px] uppercase tracking-[0.06em] text-ink-mute">Closing</span><span class="tabular text-[15px] font-semibold text-ink">{{ parsed.closing?.toLocaleString() ?? '—' }}</span></div>
            <div class="flex flex-col gap-0.5"><span class="text-[10.5px] uppercase tracking-[0.06em] text-ink-mute">Pages</span><span class="tabular text-[15px] font-semibold text-ink">{{ parsed.pages }}</span></div>
            <div class="flex flex-col gap-0.5"><span class="text-[10.5px] uppercase tracking-[0.06em] text-ink-mute">From</span><span class="font-medium text-ink">{{ parsed.rows[0]?.occurredAt || '—' }}</span></div>
            <div class="flex flex-col gap-0.5"><span class="text-[10.5px] uppercase tracking-[0.06em] text-ink-mute">To</span><span class="font-medium text-ink">{{ parsed.rows[parsed.rows.length - 1]?.occurredAt || '—' }}</span></div>
          </div>

          <div class="flex flex-wrap items-center gap-2 rounded-control border border-rule bg-page-sunk px-3.5 py-2.5 text-[12.5px]">
            <span class="stamp" :class="parsed.valid ? 'border-credit/40 text-credit' : 'border-debit/40 text-debit'">
              {{ parsed.valid ? 'opening + credits − debits = closing' : 'balances do not match' }}
            </span>
            <span class="text-ink-faint">·</span>
            <span class="text-ink-mute">Book into</span>
            <span class="font-medium text-ink">{{ accounts.find((a) => a.id === accountId)?.nickname || accounts.find((a) => a.id === accountId)?.bankName || '—' }}</span>
          </div>
          <p v-if="parsed.missingRefs" class="m-0 text-[11.5px] text-ink-mute">
            {{ parsed.missingRefs }} {{ parsed.missingRefs === 1 ? 'row has' : 'rows have' }} no reference number — the statement leaves it blank for those.
          </p>

          <div v-if="checking" class="rounded-control border border-rule bg-page-sunk px-3.5 py-2.5 text-[12.5px] text-ink-mute">
            Checking for transactions already in this account…
          </div>
          <div v-else-if="duplicateCount > 0" class="flex flex-col gap-2 rounded-control border border-stamp/40 bg-stamp/5 px-3.5 py-3">
            <p class="m-0 text-[12.5px] text-ink">
              <b>{{ duplicateCount }}</b> of {{ parsed.rows.length.toLocaleString() }} rows already exist in this account.
            </p>
            <ul class="m-0 flex list-none flex-col gap-0.5 p-0 text-[11.5px] text-ink-mute">
              <li v-for="(r, i) in duplicateSamples" :key="i" class="truncate">
                {{ r.occurredAt }} · {{ r.name }} · {{ r.amount.toLocaleString() }}
              </li>
            </ul>
            <label class="mt-1 flex items-center gap-2 text-[12.5px] text-ink">
              <input v-model="importAll" type="checkbox" class="checkbox checkbox-sm" />
              Add the duplicates too
            </label>
            <span class="text-[11.5px] text-ink-mute">Leave this off to skip them.</span>
          </div>

          <div class="overflow-hidden rounded-control border border-rule">
            <table class="w-full text-[12px]">
              <thead class="bg-page-sunk text-left text-[10.5px] uppercase tracking-[0.06em] text-ink-mute">
                <tr><th class="px-2.5 py-2">Date</th><th class="px-2.5 py-2">Name</th><th class="px-2.5 py-2">Reference</th><th class="px-2.5 py-2 text-right">Amount</th></tr>
              </thead>
              <tbody>
                <tr v-for="(r, i) in parsed.rows.slice(0, 6)" :key="i" class="border-t border-rule-soft">
                  <td class="whitespace-nowrap px-2.5 py-1.5 text-ink-mute">{{ r.occurredAt }}</td>
                  <td class="max-w-[340px] truncate px-2.5 py-1.5 text-ink-soft">{{ r.name }}</td>
                  <td class="whitespace-nowrap px-2.5 py-1.5 font-mono text-[11px] text-ink-mute">{{ r.refNo || '—' }}</td>
                  <td class="tabular whitespace-nowrap px-2.5 py-1.5 text-right" :class="r.type === 'DEBIT' ? 'text-debit' : 'text-credit'">
                    {{ r.type === 'DEBIT' ? '−' : '+' }}{{ r.amount.toLocaleString() }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p class="m-0 text-[11.5px] text-ink-mute">
            {{ importing ? `Importing ${progress} / ${parsed.rows.length}…` : `Book ${rowsToImport.toLocaleString()} of ${parsed.rows.length.toLocaleString()} rows with the statement's own names and dates.` }}
          </p>
        </template>
      </div>

      <div class="mt-3 flex shrink-0 items-center justify-between gap-2 border-t border-rule pt-[14px]">
        <button v-if="step === 0" type="button" class="btn btn-sm btn-ghost" @click="emit('close')">Cancel</button>
        <button v-else type="button" class="btn btn-sm btn-ghost" :disabled="importing" @click="back">Back</button>

        <div class="flex items-center gap-2">
          <AppButton v-if="step < 2" size="sm" :disabled="!canContinue" @click="next">Continue</AppButton>
          <AppButton v-else size="sm" :disabled="!canContinue" :loading="importing" @click="importRows">
            {{ importing ? 'Importing…' : `Import ${rowsToImport.toLocaleString()} transactions` }}
          </AppButton>
        </div>
      </div>
    </div>
  </AppModal>
</template>
