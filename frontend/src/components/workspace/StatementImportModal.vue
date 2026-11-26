<script setup lang="ts">
import { computed, ref } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppSteps from '@/components/ui/AppSteps.vue';
import { transactions } from '@/lib/api/client';
import {
  readStatementDocument,
  parseDocument,
  parseStatement,
  defaultMapping,
  type StatementParse,
  type StatementDocument,
  type StatementColumn,
  type ColumnMapping,
  type ColumnRole,
  type StatementLayout
} from '@/lib/utils/statementPdf';

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
const doc = ref<StatementDocument | null>(null);
const mapping = ref<ColumnMapping | null>(null);
const fallbackLayout = ref<'auto' | StatementLayout>('auto');
const duplicates = ref<boolean[]>([]);
const importAll = ref(false);
const checking = ref(false);

const columns = computed<StatementColumn[]>(() => parsed.value?.columns ?? []);

const ROLE_LABELS: Record<ColumnRole, string> = {
  date: 'Date',
  name: 'Name',
  ref: 'Reference',
  debit: 'Debit',
  credit: 'Credit',
  amount: 'Amount',
  balance: 'Balance',
  ignore: 'Not used'
};

const roleOptions = computed<Array<{ value: ColumnRole; label: string }>>(() => {
  const base: ColumnRole[] = ['date', 'name', 'ref'];
  if (mapping.value?.mode === 'one') base.push('amount');
  else base.push('debit', 'credit');
  base.push('balance', 'ignore');
  return base.map((value) => ({ value, label: ROLE_LABELS[value] }));
});

function roleOf(key: string): ColumnRole {
  const m = mapping.value;
  if (!m) return 'ignore';
  if (m.date === key) return 'date';
  if (m.name === key) return 'name';
  if (m.ref === key) return 'ref';
  if (m.mode === 'two' && m.debit === key) return 'debit';
  if (m.mode === 'two' && m.credit === key) return 'credit';
  if (m.mode === 'one' && m.amount === key) return 'amount';
  if (m.balance === key) return 'balance';
  return 'ignore';
}

const unmapped = computed(() => columns.value.filter((c) => roleOf(c.key) === 'ignore'));
const mappedCount = computed(() => columns.value.length - unmapped.value.length);

function roleClass(role: ColumnRole): string {
  if (role === 'debit') return 'border-debit/40 bg-debit/5 text-debit';
  if (role === 'credit') return 'border-credit/40 bg-credit/5 text-credit';
  if (role === 'ignore') return 'border-dashed border-rule-strong text-ink-mute';
  return 'border-accent/40 bg-accent-wash/60 text-accent';
}

function cellFor(col: StatementColumn, row: StatementParse['rows'][number]): string {
  const role = roleOf(col.key);
  switch (role) {
    case 'date':
      return row.occurredAt;
    case 'name':
      return row.name;
    case 'ref':
      return row.refNo || '—';
    case 'debit':
      return row.type === 'DEBIT' ? row.amount.toLocaleString() : '';
    case 'credit':
      return row.type === 'CREDIT' ? row.amount.toLocaleString() : '';
    case 'amount':
      return `${row.type === 'DEBIT' ? '−' : '+'}${row.amount.toLocaleString()}`;
    case 'balance':
      return row.balanceAfter === undefined ? '—' : row.balanceAfter.toLocaleString();
    default:
      return '';
  }
}

function cellTone(col: StatementColumn, row: StatementParse['rows'][number]): string {
  const role = roleOf(col.key);
  if (role === 'debit') return 'text-debit';
  if (role === 'credit') return 'text-credit';
  if (role === 'amount') return row.type === 'DEBIT' ? 'text-debit' : 'text-credit';
  return 'text-ink-soft';
}

const previewRows = computed(() => parsed.value?.rows.slice(0, 5) ?? []);

const canContinue = computed(() => {
  if (step.value === 0) return !!pickedFile.value && !parsing.value;
  if (step.value === 1) return !!parsed.value?.valid;
  if (step.value === 2)
    return !!parsed.value?.valid && !!accountId.value && !importing.value && !checking.value && rowsToImport.value > 0;
  return !!parsed.value?.valid && !!accountId.value && !importing.value;
});

function reparse() {
  if (!doc.value) return;
  const result = parseDocument(doc.value, mapping.value ?? undefined);
  parsed.value = result;
  duplicates.value = [];
  importAll.value = false;
  error.value = result.valid ? '' : result.problem || 'Could not validate this statement.';
}

async function parsePicked() {
  if (!pickedFile.value) return;
  error.value = '';
  parsing.value = true;
  try {
    doc.value = await readStatementDocument(pickedFile.value);
    const result = parseDocument(doc.value);
    parsed.value = result;
    mapping.value = result.mapping;
    if (!result.valid) error.value = result.problem || 'Could not validate this statement.';
  } catch (e: any) {
    error.value = e?.message || 'Could not read this PDF.';
    parsed.value = null;
    doc.value = null;
    mapping.value = null;
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
  doc.value = null;
  mapping.value = null;
  fallbackLayout.value = 'auto';
  step.value = 0;
  await parsePicked();
  input.value = '';
}

function setRole(key: string, role: ColumnRole) {
  const m = mapping.value;
  if (!m) return;
  const next: ColumnMapping = { ...m };
  if (role === 'ignore') {
    if (next.date === key) return;
    if (next.balance === key) return;
    if (next.name === key) next.name = '';
    if (next.ref === key) next.ref = '';
    if (next.debit === key) next.debit = undefined;
    if (next.credit === key) next.credit = undefined;
    if (next.amount === key) next.amount = undefined;
  } else if (role === 'debit' || role === 'credit') {
    next.mode = 'two';
    next[role] = key;
    next.amount = undefined;
    next.direction = undefined;
  } else if (role === 'amount') {
    next.mode = 'one';
    next.amount = key;
    next.debit = undefined;
    next.credit = undefined;
    next.direction = next.direction ?? 'balance';
  } else {
    next[role] = key;
  }
  mapping.value = next;
  reparse();
}

function setMode(mode: 'one' | 'two') {
  const m = mapping.value;
  if (!m || m.mode === mode) return;
  const next: ColumnMapping = { ...m };
  if (mode === 'two') {
    const used = new Set([next.date, next.name, next.ref, next.balance].filter(Boolean));
    const free = columns.value.find((c) => !used.has(c.key) && !/balance/i.test(c.label));
    next.mode = 'two';
    next.debit = next.amount ?? next.debit;
    next.credit = next.credit ?? free?.key ?? next.amount;
    next.amount = undefined;
    next.direction = undefined;
  } else {
    next.mode = 'one';
    next.amount = next.debit ?? next.amount ?? next.credit;
    next.debit = undefined;
    next.credit = undefined;
    next.direction = 'balance';
  }
  mapping.value = next;
  reparse();
}

function setDirection(direction: 'marker' | 'balance') {
  if (!mapping.value) return;
  mapping.value = { ...mapping.value, direction };
  reparse();
}

function resetMapping() {
  const detected = defaultMapping(columns.value);
  if (!detected) return;
  mapping.value = detected;
  reparse();
}

async function changeLayout() {
  duplicates.value = [];
  importAll.value = false;
  if (!pickedFile.value || !doc.value) return;
  parsing.value = true;
  const result = parseStatement(doc.value.lines, fallbackLayout.value === 'auto' ? undefined : fallbackLayout.value);
  parsed.value = { ...result, pages: doc.value.pages };
  if (!result.valid) error.value = result.problem || 'Could not validate this statement.';
  parsing.value = false;
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
  const rows = importAll.value ? parsed.value.rows : parsed.value.rows.filter((_r, i) => !duplicates.value[i]);
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
        <p
          v-if="error"
          role="alert"
          class="m-0 shrink-0 rounded-control border border-debit/30 bg-debit/5 px-3 py-2 text-[12.5px] text-debit"
        >
          {{ error }}
        </p>
        <template v-if="step === 0">
          <p class="m-0 text-[13px] leading-relaxed text-ink-mute">
            Choose a statement PDF. Rows are read in your browser, checked against the statement's own balances, then
            booked. Nothing leaves this device until you press Import.
          </p>

          <label
            class="flex cursor-pointer flex-col items-center gap-3 rounded-folio border-[1.5px] border-dashed border-rule-strong bg-page-sunk px-6 py-7 text-center transition-colors hover:border-accent/60 hover:bg-accent-wash/40 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-accent/40"
          >
            <span
              class="grid h-11 w-11 place-items-center rounded-folio bg-accent-wash text-[18px] text-accent"
              aria-hidden="true"
              >↑</span
            >
            <span class="flex flex-col gap-0.5">
              <span class="text-[13.5px] font-semibold text-ink">{{
                fileName || 'Drop a PDF here, or choose a file'
              }}</span>
              <span class="text-[12px] text-ink-mute">
                {{ parsing ? 'Reading…' : 'Bank statement · PDF only' }}
              </span>
            </span>
            <input type="file" accept="application/pdf" class="sr-only" @change="onFile" />
          </label>

          <ul class="m-0 flex list-none flex-col gap-1.5 p-0 text-[12px] text-ink-mute">
            <li class="flex items-start gap-2">
              <span class="mt-[6px] h-1 w-1 shrink-0 rounded-full bg-ink-faint" aria-hidden="true" />Parsed in your
              browser — the PDF is not uploaded.
            </li>
            <li class="flex items-start gap-2">
              <span class="mt-[6px] h-1 w-1 shrink-0 rounded-full bg-ink-faint" aria-hidden="true" />Every row is
              checked against the statement's own balances.
            </li>
            <li class="flex items-start gap-2">
              <span class="mt-[6px] h-1 w-1 shrink-0 rounded-full bg-ink-faint" aria-hidden="true" />Nothing is booked
              until you press Import on the last step.
            </li>
          </ul>

          <label class="flex flex-col gap-1.5">
            <span class="text-[11px] font-medium uppercase tracking-[0.1em] text-ink-mute">Book into</span>
            <select v-model="accountId" class="select select-bordered h-10 w-full text-[13.5px]">
              <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.nickname || a.bankName }}</option>
            </select>
          </label>

          <p v-if="parsed" class="m-0 flex flex-wrap items-center gap-2 text-[12.5px] text-ink-soft">
            <span>{{ fileName }}</span><span class="text-ink-faint">·</span>
            <span class="num">{{ parsed.rows.length.toLocaleString() }} rows</span><span class="text-ink-faint">·</span>
            <span class="num">{{ parsed.pages }} pages</span><span class="text-ink-faint">·</span>
            <span class="stamp" :class="parsed.valid ? 'border-credit/40 text-credit' : 'border-debit/40 text-debit'">
              {{ parsed.valid ? 'balances match' : 'balances do not match' }}
            </span>
          </p>
        </template>
        <template v-else-if="step === 1 && parsed">
          <div
            class="flex flex-wrap items-center gap-2 rounded-control border border-rule bg-page-sunk px-3.5 py-2.5 text-[12.5px]"
          >
            <span class="font-mono text-ink-soft">{{ fileName }}</span>
            <span class="text-ink-faint">·</span>
            <span class="num">{{ parsed.rows.length.toLocaleString() }} rows</span>
            <span class="text-ink-faint">·</span>
            <span class="num">{{ parsed.pages }} pages</span>
            <span class="stamp accent ml-auto">
              {{ columns.length ? `${columns.length} columns detected` : 'automatic layout' }}
            </span>
          </div>

          <section
            v-if="columns.length && mapping"
            class="shrink-0 overflow-hidden rounded-control border border-rule"
            aria-labelledby="map-columns-title"
          >
            <div class="flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-rule-soft px-3 py-2.5">
              <h3 id="map-columns-title" class="m-0 text-[11px] font-semibold uppercase tracking-[0.1em] text-ink-mute">
                Match the statement columns
              </h3>
              <span class="inline-flex items-center gap-2 text-[11.5px] text-ink-mute">
                <span class="h-1 w-14 overflow-hidden rounded-full bg-page-sunk" aria-hidden="true">
                  <span
                    class="block h-full bg-accent transition-[width]"
                    :style="{ width: `${Math.round((mappedCount / columns.length) * 100)}%` }"
                  />
                </span>
                <span class="num">{{ mappedCount }} of {{ columns.length }} mapped</span>
              </span>
              <button
                type="button"
                class="rounded-control px-1.5 py-1 text-[11px] font-semibold text-accent hover:bg-accent-wash focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-accent"
                @click="resetMapping"
              >
                Reset to detected
              </button>
              <div
                class="ml-auto inline-flex gap-0.5 rounded-control border border-rule bg-page-sunk p-0.5"
                role="group"
                aria-label="Amount layout"
              >
                <button
                  type="button"
                  class="rounded-[3px] px-2.5 py-1.5 text-[11px] font-semibold transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-accent"
                  :class="
                    mapping.mode === 'two'
                      ? 'border border-rule bg-page text-ink shadow-sm'
                      : 'text-ink-mute hover:text-ink'
                  "
                  :aria-pressed="mapping.mode === 'two'"
                  @click="setMode('two')"
                >
                  Debit + Credit
                </button>
                <button
                  type="button"
                  class="rounded-[3px] px-2.5 py-1.5 text-[11px] font-semibold transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-accent"
                  :class="
                    mapping.mode === 'one'
                      ? 'border border-rule bg-page text-ink shadow-sm'
                      : 'text-ink-mute hover:text-ink'
                  "
                  :aria-pressed="mapping.mode === 'one'"
                  @click="setMode('one')"
                >
                  One amount
                </button>
              </div>
            </div>

            <div
              class="overflow-x-auto"
              tabindex="0"
              role="region"
              aria-label="Statement columns, scroll horizontally to see them all"
            >
              <table class="w-full min-w-[780px] border-collapse text-[11.5px]">
                <thead>
                  <tr class="bg-page-sunk text-left text-[9.5px] uppercase tracking-[0.1em]">
                    <th
                      v-for="col in columns"
                      :key="col.key"
                      scope="col"
                      class="whitespace-nowrap border-b border-rule-soft px-2.5 py-2 font-semibold"
                      :class="roleOf(col.key) === 'ignore' ? 'text-ink-faint' : 'text-accent'"
                    >
                      {{ col.key }} · {{ col.label }}
                    </th>
                  </tr>
                  <tr>
                    <th
                      v-for="col in columns"
                      :key="col.key"
                      scope="col"
                      class="border-b border-rule px-1.5 pb-2 pt-1.5 text-left"
                    >
                      <div
                        class="relative inline-flex rounded-control focus-within:outline focus-within:outline-2 focus-within:outline-offset-1 focus-within:outline-accent"
                      >
                        <span
                          class="pointer-events-none inline-flex h-11 items-center gap-1.5 rounded-control border px-2 text-[10.5px] font-semibold md:h-7"
                          :class="roleClass(roleOf(col.key))"
                        >
                          {{ ROLE_LABELS[roleOf(col.key)] }}
                          <svg viewBox="0 0 10 6" class="h-[5px] w-[9px] opacity-60" fill="none" aria-hidden="true">
                            <path d="M1 1l4 4 4-4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
                          </svg>
                        </span>
                        <select
                          class="absolute inset-0 h-full w-full cursor-pointer appearance-none opacity-0"
                          :value="roleOf(col.key)"
                          :aria-label="`Column ${col.key}, ${col.label}. Choose its role.`"
                          @change="setRole(col.key, ($event.target as HTMLSelectElement).value as ColumnRole)"
                        >
                          <option v-for="opt in roleOptions" :key="opt.value" :value="opt.value">
                            {{ opt.label }}
                          </option>
                        </select>
                      </div>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(r, i) in previewRows" :key="i" class="border-t border-rule-soft">
                    <td
                      v-for="col in columns"
                      :key="col.key"
                      class="max-w-[220px] truncate whitespace-nowrap px-2.5 py-1.5"
                      :class="[
                        cellTone(col, r),
                        ['date', 'ref'].includes(roleOf(col.key)) ? 'font-mono text-[10.5px]' : ''
                      ]"
                    >
                      {{ cellFor(col, r) || '—' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div
              class="flex flex-wrap items-center gap-2 border-t border-rule-soft bg-page-sunk px-3 py-2 text-[11px] text-ink-mute"
            >
              <span class="font-semibold uppercase tracking-[0.1em]">Unmapped</span>
              <span
                v-for="col in unmapped"
                :key="col.key"
                class="inline-flex items-center gap-1.5 rounded-control border border-dashed border-rule-strong bg-page px-2 py-1"
              >
                <b class="font-mono text-[9px] text-ink-faint">{{ col.key }}</b>{{ col.label }}
              </span>
              <span v-if="!unmapped.length" class="text-ink-faint">none</span>
              <label v-if="mapping.mode === 'one'" class="inline-flex items-center gap-2">
                <span
                  class="inline-flex items-center gap-1.5 rounded-control border border-dashed border-rule-strong bg-page px-2 py-1"
                >
                  <b class="font-mono text-[9px] text-ink-faint">↕</b>Direction
                </span>
                <select
                  class="select select-bordered h-11 min-h-0 rounded-control px-2 text-[11px] md:h-7"
                  :value="mapping.direction"
                  aria-label="Where the direction comes from"
                  @change="setDirection(($event.target as HTMLSelectElement).value as 'marker' | 'balance')"
                >
                  <option value="balance">from the closing balance</option>
                  <option value="marker">Dr / Cr marker</option>
                </select>
              </label>
              <span class="ml-auto">
                <template v-if="mapping.mode === 'one' && mapping.direction === 'balance'"
                  >The opening is inferred from row 1.
                </template>
                Tap a role to change it. Name comes from the narration; unmapped columns are ignored.
              </span>
            </div>

            <div
              class="flex flex-wrap items-center gap-x-4 gap-y-1 border-t border-rule-soft px-3 py-2.5 text-[11.5px]"
            >
              <span class="inline-flex items-center gap-1.5 text-ink-soft"
                >✓ Opening {{ parsed.opening?.toLocaleString() ?? '—' }}</span
              >
              <span class="inline-flex items-center gap-1.5 text-ink-soft"
                >✓ Closing {{ parsed.closing?.toLocaleString() ?? '—' }}</span
              >
              <span class="inline-flex items-center gap-1.5 text-ink-soft"
                >✓ {{ parsed.rows.length.toLocaleString() }} rows</span
              >
              <span class="inline-flex items-center gap-1.5 text-ink-soft">{{ parsed.missingRefs }} missing refs</span>
              <span
                class="ml-auto inline-flex items-center gap-1.5 font-semibold"
                :class="parsed.valid ? 'text-credit' : 'text-debit'"
              >
                {{ parsed.valid ? '✓ Balances reconcile' : '✕ Balances do not reconcile' }}
              </span>
            </div>
          </section>

          <div v-else class="flex flex-col gap-1.5">
            <label class="flex flex-col gap-1.5">
              <span class="text-[11px] font-medium uppercase tracking-[0.1em] text-ink-mute">Column layout</span>
              <select
                v-model="fallbackLayout"
                class="select select-bordered h-10 w-full text-[13.5px]"
                @change="changeLayout"
              >
                <option value="auto">Detect automatically</option>
                <option value="withdrawal-deposit">Withdrawal · Deposit · Balance</option>
                <option value="amount-marker">Amount · Dr/Cr marker · Balance</option>
                <option value="amount-delta">Amount · Balance (direction from balance)</option>
                <option value="amount-only">Amount · Dr/Cr, no balance</option>
              </select>
            </label>
            <span class="text-[11.5px] text-ink-mute">
              This statement prints no column headers, so the amounts are read by layout. The date is the first date
              column; the balance is the last amount.
            </span>
          </div>
        </template>
        <template v-else-if="step === 2 && parsed">
          <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <div class="flex flex-col gap-0.5">
              <span class="text-[10.5px] uppercase tracking-[0.1em] text-ink-mute">Rows</span
              ><span class="num text-[15px] font-semibold text-ink">{{ parsed.rows.length.toLocaleString() }}</span>
            </div>
            <div class="flex flex-col gap-0.5">
              <span class="text-[10.5px] uppercase tracking-[0.1em] text-ink-mute">Opening</span
              ><span class="num text-[15px] font-semibold text-ink">{{ parsed.opening?.toLocaleString() ?? '—' }}</span>
            </div>
            <div class="flex flex-col gap-0.5">
              <span class="text-[10.5px] uppercase tracking-[0.1em] text-ink-mute">Credits</span
              ><span class="num text-[15px] font-semibold text-credit">{{ parsed.credits.toLocaleString() }}</span>
            </div>
            <div class="flex flex-col gap-0.5">
              <span class="text-[10.5px] uppercase tracking-[0.1em] text-ink-mute">Debits</span
              ><span class="num text-[15px] font-semibold text-debit">{{ parsed.debits.toLocaleString() }}</span>
            </div>
            <div class="flex flex-col gap-0.5">
              <span class="text-[10.5px] uppercase tracking-[0.1em] text-ink-mute">Closing</span
              ><span class="num text-[15px] font-semibold text-ink">{{ parsed.closing?.toLocaleString() ?? '—' }}</span>
            </div>
            <div class="flex flex-col gap-0.5">
              <span class="text-[10.5px] uppercase tracking-[0.1em] text-ink-mute">Pages</span
              ><span class="num text-[15px] font-semibold text-ink">{{ parsed.pages }}</span>
            </div>
            <div class="flex flex-col gap-0.5">
              <span class="text-[10.5px] uppercase tracking-[0.1em] text-ink-mute">From</span
              ><span class="font-medium text-ink">{{ parsed.rows[0]?.occurredAt || '—' }}</span>
            </div>
            <div class="flex flex-col gap-0.5">
              <span class="text-[10.5px] uppercase tracking-[0.1em] text-ink-mute">To</span
              ><span class="font-medium text-ink">{{ parsed.rows[parsed.rows.length - 1]?.occurredAt || '—' }}</span>
            </div>
          </div>

          <div
            class="flex flex-wrap items-center gap-2 rounded-control border border-rule bg-page-sunk px-3.5 py-2.5 text-[12.5px]"
          >
            <span class="stamp" :class="parsed.valid ? 'border-credit/40 text-credit' : 'border-debit/40 text-debit'">
              {{ parsed.valid ? 'opening + credits − debits = closing' : 'balances do not match' }}
            </span>
            <span class="text-ink-faint">·</span>
            <span class="text-ink-mute">Book into</span>
            <span class="font-medium text-ink">{{
              accounts.find((a) => a.id === accountId)?.nickname ||
              accounts.find((a) => a.id === accountId)?.bankName ||
              '—'
            }}</span>
          </div>
          <p v-if="parsed.missingRefs" class="m-0 text-[11.5px] text-ink-mute">
            {{ parsed.missingRefs }} {{ parsed.missingRefs === 1 ? 'row has' : 'rows have' }} no reference number — the
            statement leaves it blank for those.
          </p>

          <div
            v-if="checking"
            class="rounded-control border border-rule bg-page-sunk px-3.5 py-2.5 text-[12.5px] text-ink-mute"
          >
            Checking for transactions already in this account…
          </div>
          <div
            v-else-if="duplicateCount > 0"
            class="flex flex-col gap-2 rounded-control border border-stamp/40 bg-stamp/5 px-3.5 py-3"
          >
            <p class="m-0 text-[12.5px] text-ink">
              <b>{{ duplicateCount }}</b> of {{ parsed.rows.length.toLocaleString() }} rows already exist in this
              account.
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
              <thead class="bg-page-sunk text-left text-[10.5px] uppercase tracking-[0.1em] text-ink-mute">
                <tr>
                  <th class="px-2.5 py-2">Date</th>
                  <th class="px-2.5 py-2">Name</th>
                  <th class="px-2.5 py-2">Reference</th>
                  <th class="px-2.5 py-2 text-right">Amount</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(r, i) in parsed.rows.slice(0, 6)" :key="i" class="border-t border-rule-soft">
                  <td class="whitespace-nowrap px-2.5 py-1.5 text-ink-mute">{{ r.occurredAt }}</td>
                  <td class="max-w-[340px] truncate px-2.5 py-1.5 text-ink-soft">{{ r.name }}</td>
                  <td class="whitespace-nowrap px-2.5 py-1.5 font-mono text-[11px] text-ink-mute">
                    {{ r.refNo || '—' }}
                  </td>
                  <td
                    class="num whitespace-nowrap px-2.5 py-1.5 text-right"
                    :class="r.type === 'DEBIT' ? 'text-debit' : 'text-credit'"
                  >
                    {{ r.type === 'DEBIT' ? '−' : '+' }}{{ r.amount.toLocaleString() }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p class="m-0 text-[11.5px] text-ink-mute">
            {{
              importing
                ? `Importing ${progress} / ${parsed.rows.length}…`
                : `Book ${rowsToImport.toLocaleString()} of ${parsed.rows.length.toLocaleString()} rows with the statement's own names and dates.`
            }}
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
