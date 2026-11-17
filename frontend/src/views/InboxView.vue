<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { categories, mail } from '@/lib/api/client';
import { formatCurrency, formatDate } from '@/lib/utils/format';
import MailboxFormModal from '@/components/mail/MailboxFormModal.vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import { Loader2, Inbox as InboxIcon, RefreshCw, Settings2, Plus, Mail } from '@lucide/vue';

interface MailAccount {
  id: string;
  label: string;
  imapHost: string;
  imapPort: number;
  username: string;
  enabled: boolean;
  lastPolledAt?: string | null;
  lastError: string;
}

interface MailMessage {
  id: string;
  messageId: string;
  mailAccountId: string;
  fromAddr: string;
  subject: string;
  receivedAt?: string | null;
  status: string;
  parsedJson?: string | null;
  confidence: number;
  rawText?: string | null;
  transactionId?: string | null;
}

interface Extraction {
  amount?: number;
  direction?: string;
  occurredAt?: string | null;
  merchant?: string;
  accountHint?: string | null;
  confidence?: number;
}

interface Category {
  id: string;
  name: string;
}

const router = useRouter();
const accounts = ref<MailAccount[]>([]);
const messages = ref<MailMessage[]>([]);
const cats = ref<Category[]>([]);
const loading = ref(true);
const error = ref('');
const notice = ref('');
const checking = ref(false);
const busy = ref(false);
const filter = ref<'review' | 'unparsed' | 'auto' | 'all'>('review');
const activeAccount = ref('all');
const search = ref('');
const selectedId = ref('');
const showMailboxForm = ref(false);
const editAccount = ref<MailAccount | null>(null);
const form = ref({
  name: '',
  amount: '',
  type: 'DEBIT',
  date: '',
  accountId: '',
  categoryIds: [] as string[]
});
const formErrors = ref<string[]>([]);
let refreshTimer: number | null = null;

const FILTERS = [
  { key: 'review', label: 'Needs review' },
  { key: 'unparsed', label: 'Unparsed' },
  { key: 'auto', label: 'Auto' },
  { key: 'all', label: 'All' }
] as const;

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase();
  return messages.value.filter((m) => {
    if (activeAccount.value !== 'all' && m.mailAccountId !== activeAccount.value) return false;
    if (filter.value === 'review' && !['PENDING', 'UNPARSED'].includes(m.status)) return false;
    if (filter.value === 'unparsed' && m.status !== 'UNPARSED') return false;
    if (filter.value === 'auto' && !['AUTO', 'REJECTED'].includes(m.status)) return false;
    if (q && !(`${m.subject} ${m.fromAddr}`.toLowerCase().includes(q))) return false;
    return true;
  });
});

const selected = computed(() => messages.value.find((m) => m.id === selectedId.value) || null);
const parsed = computed<Extraction | null>(() => {
  if (!selected.value?.parsedJson) return null;
  try {
    return JSON.parse(selected.value.parsedJson) as Extraction;
  } catch {
    return null;
  }
});
const actionable = computed(() => {
  if (!selected.value) return false;
  if (['PENDING', 'UNPARSED'].includes(selected.value.status)) return true;
  return selected.value.status === 'AUTO' && !selected.value.transactionId;
});
const accountOptions = computed(() => accounts.value.map((a) => ({ value: a.id, label: a.label || a.username })));

function mailboxName(id: string): string {
  const a = accounts.value.find((x) => x.id === id);
  return a ? a.label || a.username : 'Mailbox';
}

function parse(m: MailMessage): Extraction | null {
  if (!m.parsedJson) return null;
  try { return JSON.parse(m.parsedJson) as Extraction; } catch { return null; }
}

function amountOf(m: MailMessage): string {
  const p = parse(m);
  if (!p?.amount) return '';
  const sign = p.direction === 'CREDIT' ? '+' : '−';
  return `${sign} ${formatCurrency(p.amount)}`;
}

function stampClass(status: string): string {
  if (status === 'AUTO') return 'stamp auto';
  if (status === 'UNPARSED') return 'stamp unparsed';
  if (status === 'REJECTED') return 'stamp';
  if (status === 'IGNORED') return 'stamp';
  return 'stamp pending';
}

function prepForm(m: MailMessage) {
  const p = parse(m);
  form.value = {
    name: p?.merchant || '',
    amount: p?.amount ? String(p.amount) : '',
    type: p?.direction || 'DEBIT',
    date: p?.occurredAt ? String(p.occurredAt).slice(0, 10) : '',
    accountId: '',
    categoryIds: []
  };
  formErrors.value = [];
}

function select(m: MailMessage) {
  selectedId.value = m.id;
  prepForm(m);
  error.value = '';
  notice.value = '';
}

async function load() {
  loading.value = true;
  error.value = '';
  try {
    const [acc, msgs, catRes] = await Promise.all([
      mail().listAccounts({}),
      mail().listMessages({ status: 'all', limit: 300 }),
      categories().listCategories({ pageSize: 200 })
    ]);
    accounts.value = acc.accounts || [];
    messages.value = msgs.messages || [];
    cats.value = catRes.categories || [];
    if (selectedId.value && !messages.value.some((m) => m.id === selectedId.value)) {
      selectedId.value = '';
    }
    if (!selectedId.value) {
      const first = filtered.value[0];
      if (first) select(first);
    }
  } catch (e: any) {
    error.value = e?.message || 'Failed to load the inbox';
  }
  loading.value = false;
}

function startRefresh() {
  if (refreshTimer) window.clearInterval(refreshTimer);
  let ticks = 0;
  refreshTimer = window.setInterval(async () => {
    ticks += 1;
    await load();
    if (ticks >= 5 || !checking.value) {
      if (refreshTimer) window.clearInterval(refreshTimer);
      refreshTimer = null;
      checking.value = false;
    }
  }, 3000);
}

async function poll(force = false) {
  checking.value = true;
  error.value = '';
  try {
    const res = await mail().pollAll(force ? { force: 1 } : {});
    if (res?.status === 'throttled') {
      notice.value = 'Mail was checked recently.';
      checking.value = false;
      await load();
      return;
    }
    startRefresh();
  } catch (e: any) {
    checking.value = false;
    error.value = e?.message || 'Could not start the mail check';
  }
}

function validate(): string[] {
  const errs: string[] = [];
  if (!form.value.name.trim()) errs.push('Name is required');
  if (!form.value.amount || Number(form.value.amount) <= 0) errs.push('Amount must be greater than zero');
  if (!form.value.date) errs.push('Date is required');
  if (!form.value.accountId) errs.push('Choose the account');
  return errs;
}

async function approve() {
  if (!selected.value) return;
  formErrors.value = validate();
  if (formErrors.value.length) return;
  busy.value = true;
  error.value = '';
  try {
    const res = await mail().act({
      messages: [{
        id: selected.value.id,
        action: 'APPROVE',
        name: form.value.name.trim(),
        amount: Number(form.value.amount),
        type: form.value.type,
        occurredAt: form.value.date,
        accountId: form.value.accountId,
        categoryIds: form.value.categoryIds
      }]
    });
    if (res.failed?.length) {
      error.value = res.failed[0].error;
    } else {
      notice.value = 'Transaction created.';
      selectedId.value = '';
      await load();
    }
  } catch (e: any) {
    error.value = e?.message || 'Could not approve this mail';
  }
  busy.value = false;
}

async function reject() {
  if (!selected.value) return;
  busy.value = true;
  error.value = '';
  try {
    const res = await mail().act({ messages: [{ id: selected.value.id, action: 'REJECT' }] });
    if (res.failed?.length) error.value = res.failed[0].error;
    else {
      notice.value = 'Mail rejected.';
      selectedId.value = '';
      await load();
    }
  } catch (e: any) {
    error.value = e?.message || 'Could not reject this mail';
  }
  busy.value = false;
}

function move(delta: number) {
  const list = filtered.value;
  if (!list.length) return;
  const idx = list.findIndex((m) => m.id === selectedId.value);
  const next = list[Math.max(0, Math.min(list.length - 1, idx + delta))];
  if (next) select(next);
}

function onKey(e: KeyboardEvent) {
  const tag = (e.target as HTMLElement)?.tagName;
  if (['INPUT', 'TEXTAREA', 'SELECT'].includes(tag)) return;
  if (e.key === 'a' && actionable.value) { e.preventDefault(); approve(); }
  if (e.key === 'r' && actionable.value) { e.preventDefault(); reject(); }
  if (e.key === 'j') { e.preventDefault(); move(1); }
  if (e.key === 'k') { e.preventDefault(); move(-1); }
}

function openAdd() {
  editAccount.value = null;
  showMailboxForm.value = true;
}

onMounted(async () => {
  await load();
  poll(false);
  window.addEventListener('keydown', onKey);
});
onUnmounted(() => {
  window.removeEventListener('keydown', onKey);
  if (refreshTimer) window.clearInterval(refreshTimer);
});
</script>

<template>
  <div class="flex min-h-0 w-full flex-1 flex-col gap-3">
    <SectionHeader title="Inbox" subtitle="Review bank alerts and turn them into entries">
      <template #actions>
        <div class="flex items-center gap-2">
          <span v-if="checking" class="flex items-center gap-1.5 text-[12px] text-ink-mute">
            <Loader2 class="h-3.5 w-3.5 animate-spin" /> Checking mail…
          </span>
          <button class="ns-quiet" :disabled="checking" @click="poll(true)">
            <RefreshCw class="h-4 w-4" :class="checking ? 'animate-spin' : ''" /> Poll now
          </button>
          <button class="ns-quiet" @click="router.push('/settings?tab=mailboxes')">
            <Settings2 class="h-4 w-4" /> Mailboxes
          </button>
          <button class="ns-primary" @click="openAdd"><Plus class="h-4 w-4" /> Add mailbox</button>
        </div>
      </template>
    </SectionHeader>

    <p v-if="error" class="rounded-card border border-debit/40 px-3.5 py-2 text-[12.5px] text-debit">{{ error }}</p>
    <p v-if="notice" class="rounded-card border border-credit/40 px-3.5 py-2 text-[12.5px] text-credit">{{ notice }}</p>

    <div class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-card border border-rule bg-page">
      <div class="flex flex-wrap items-center gap-1 border-b border-rule bg-paper px-2">
        <button
          class="px-3 py-2 text-[12.5px]"
          :class="activeAccount === 'all' ? 'border-b-2 border-accent text-ink' : 'text-ink-mute hover:text-ink'"
          @click="activeAccount = 'all'"
        >All mailboxes</button>
        <button
          v-for="a in accounts"
          :key="a.id"
          class="flex items-center gap-1.5 px-3 py-2 text-[12.5px]"
          :class="activeAccount === a.id ? 'border-b-2 border-accent text-ink' : 'text-ink-mute hover:text-ink'"
          @click="activeAccount = a.id"
        >
          {{ a.label || a.username }}
          <span v-if="a.lastError" class="h-1.5 w-1.5 rounded-full bg-debit" :title="a.lastError"></span>
        </button>
        <span v-if="!accounts.length" class="px-3 py-2 text-[12.5px] text-ink-mute">No mailboxes linked yet.</span>
      </div>

      <div class="flex flex-wrap items-center gap-2 border-b border-rule-soft px-3 py-2">
        <button
          v-for="f in FILTERS"
          :key="f.key"
          class="rounded-full border px-2.5 py-1 text-[12px]"
          :class="filter === f.key ? 'border-accent text-accent' : 'border-rule text-ink-soft hover:text-ink'"
          @click="filter = f.key"
        >{{ f.label }}</button>
        <input
          v-model="search"
          type="search"
          placeholder="Search sender or subject"
          class="ml-auto w-[220px] rounded-control border border-rule bg-page px-2.5 py-1.5 text-[12.5px] text-ink outline-none focus:border-accent"
        />
      </div>

      <div class="grid min-h-0 flex-1 lg:grid-cols-[minmax(300px,400px)_1fr]">
        <div class="min-h-0 overflow-y-auto border-b border-rule lg:border-b-0 lg:border-r">
          <div v-if="loading" class="py-16 text-center text-[13px] text-ink-mute">Loading…</div>
          <div v-else-if="!filtered.length" class="flex flex-col items-center gap-2 py-16 text-center">
            <InboxIcon class="h-8 w-8 text-ink-faint" />
            <p class="text-[13px] text-ink-mute">Nothing to review here.</p>
            <p class="text-[12px] text-ink-faint">New mail appears automatically after a poll.</p>
          </div>
          <button
            v-for="m in filtered"
            :key="m.id"
            class="flex w-full gap-2.5 border-b border-rule-soft px-3.5 py-3 text-left"
            :class="selectedId === m.id ? 'bg-page shadow-[inset_2px_0_0_var(--accent)]' : 'bg-paper hover:bg-page'"
            @click="select(m)"
          >
            <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full"
              :class="m.status === 'UNPARSED' ? 'bg-debit' : m.status === 'AUTO' ? 'bg-credit' : 'bg-stamp'"></span>
            <span class="min-w-0 flex-1">
              <span class="flex items-baseline justify-between gap-2">
                <span class="truncate text-[13px] text-ink">{{ m.subject || '(no subject)' }}</span>
                <span class="mono shrink-0 text-[11.5px]" :class="(parse(m)?.direction === 'CREDIT') ? 'text-credit' : 'text-debit'">{{ amountOf(m) }}</span>
              </span>
              <span class="mt-0.5 flex items-center justify-between gap-2 text-[11.5px] text-ink-mute">
                <span class="truncate">{{ m.fromAddr }}</span>
                <span class="flex shrink-0 items-center gap-2">
                  <span>{{ m.receivedAt ? formatDate(m.receivedAt) : formatDate(m.messageId) }}</span>
                  <span :class="stampClass(m.status)">{{ m.status }}</span>
                </span>
              </span>
            </span>
          </button>
        </div>

        <div v-if="selected" class="min-h-0 overflow-y-auto px-4 py-4 lg:px-5">
          <div class="flex flex-wrap items-start justify-between gap-3 border-b border-rule pb-3">
            <div class="min-w-0">
              <h2 class="truncate font-display text-[20px] text-ink">{{ selected.subject || '(no subject)' }}</h2>
              <p class="mt-0.5 text-[12px] text-ink-mute">
                {{ selected.fromAddr }} · {{ selected.receivedAt ? formatDate(selected.receivedAt) : '—' }} · {{ mailboxName(selected.mailAccountId) }}
              </p>
            </div>
            <span :class="stampClass(selected.status)">{{ selected.status }}</span>
          </div>

          <div class="mt-3 grid gap-5 xl:grid-cols-[1fr_320px]">
            <div class="min-w-0">
              <p class="mb-1 text-[10px] uppercase tracking-[0.08em] text-ink-mute">Email</p>
              <pre class="max-h-[300px] overflow-auto whitespace-pre-wrap rounded-control border border-rule-soft bg-page-sunk p-3 font-ui text-[12.5px] leading-relaxed text-ink-soft">{{ selected.rawText || 'No body stored.' }}</pre>
              <div v-if="parsed?.accountHint" class="mt-2 text-[11.5px] text-ink-mute">
                Account hint in mail: <span class="mono">{{ parsed.accountHint }}</span>
              </div>
              <div v-if="parsed?.confidence != null" class="text-[11.5px] text-ink-mute">
                Extraction confidence: <span class="mono">{{ Math.round((parsed.confidence || 0) * 100) }}%</span>
              </div>
            </div>

            <div class="min-w-0">
              <div v-if="formErrors.length" role="alert" class="mb-3 rounded-control border border-debit/40 bg-debit/5 px-3 py-2 text-[12px] text-debit">
                <b class="mb-0.5 block">There is a problem</b>
                <ul class="list-inside list-disc">
                  <li v-for="e in formErrors" :key="e">{{ e }}</li>
                </ul>
              </div>

              <label class="mb-1 block text-[10px] uppercase tracking-[0.08em] text-ink-mute" for="mail-name">Name</label>
              <input id="mail-name" v-model="form.name" class="mb-2.5 w-full rounded-control border px-2.5 py-1.5 text-[13px]"
                :class="!form.name.trim() ? 'border-stamp/60 bg-stamp/5' : 'border-rule'" />

              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="mb-1 block text-[10px] uppercase tracking-[0.08em] text-ink-mute" for="mail-amount">Amount</label>
                  <input id="mail-amount" v-model="form.amount" type="number" step="0.01" min="0"
                    class="w-full rounded-control border px-2.5 py-1.5 text-[13px] mono"
                    :class="!form.amount ? 'border-stamp/60 bg-stamp/5' : 'border-rule'" />
                </div>
                <div>
                  <label class="mb-1 block text-[10px] uppercase tracking-[0.08em] text-ink-mute" for="mail-type">Type</label>
                  <select id="mail-type" v-model="form.type" class="w-full rounded-control border border-rule bg-page px-2 py-1.5 text-[13px]">
                    <option value="DEBIT">Debit</option>
                    <option value="CREDIT">Credit</option>
                  </select>
                </div>
              </div>

              <label class="mb-1 mt-2.5 block text-[10px] uppercase tracking-[0.08em] text-ink-mute" for="mail-date">Date</label>
              <input id="mail-date" v-model="form.date" type="date"
                class="mb-2.5 w-full rounded-control border px-2.5 py-1.5 text-[13px]"
                :class="!form.date ? 'border-stamp/60 bg-stamp/5' : 'border-rule'" />

              <label class="mb-1 block text-[10px] uppercase tracking-[0.08em] text-ink-mute" for="mail-account">Account</label>
              <select id="mail-account" v-model="form.accountId"
                class="w-full rounded-control border bg-page px-2 py-1.5 text-[13px]"
                :class="!form.accountId ? 'border-stamp/60 bg-stamp/5' : 'border-rule'">
                <option value="" disabled>Choose an account</option>
                <option v-for="a in accountOptions" :key="a.value" :value="a.value">{{ a.label }}</option>
              </select>

              <p class="mb-1 mt-2.5 text-[10px] uppercase tracking-[0.08em] text-ink-mute">Categories</p>
              <div class="flex max-h-[120px] flex-wrap gap-1.5 overflow-y-auto">
                <button
                  v-for="c in cats"
                  :key="c.id"
                  class="rounded-full border px-2 py-0.5 text-[11.5px]"
                  :class="form.categoryIds.includes(c.id) ? 'border-accent text-accent' : 'border-rule text-ink-soft'"
                  @click="form.categoryIds.includes(c.id) ? form.categoryIds = form.categoryIds.filter((id) => id !== c.id) : form.categoryIds.push(c.id)"
                >{{ c.name }}</button>
                <span v-if="!cats.length" class="text-[11.5px] text-ink-faint">No categories yet.</span>
              </div>

              <div class="mt-4 flex items-center gap-2">
                <button class="ns-primary flex-1" :disabled="busy || !actionable" @click="approve">
                  Approve<span class="kbd">A</span>
                </button>
                <button class="ns-quiet" :disabled="busy || !actionable" @click="reject">Reject<span class="kbd">R</span></button>
              </div>
              <div class="mt-2 flex gap-3 text-[11px] text-ink-faint">
                <span><span class="kbd">J</span>/<span class="kbd">K</span> next/prev</span>
                <span v-if="!actionable">Already decided</span>
              </div>
            </div>
          </div>
        </div>

        <div v-else class="flex flex-col items-center justify-center gap-2 p-10 text-center">
          <Mail class="h-8 w-8 text-ink-faint" />
          <p class="text-[13px] text-ink-mute">Select a mail to review.</p>
          <p class="text-[12px] text-ink-faint">The email and its extracted fields appear here.</p>
        </div>
      </div>
    </div>

    <MailboxFormModal
      v-if="showMailboxForm"
      :account="editAccount"
      @close="showMailboxForm = false"
      @saved="load(); poll(false)"
    />
  </div>
</template>

<style scoped>
.kbd {
  display: inline-block;
  margin-left: 6px;
  border: 1px solid var(--rule);
  border-bottom-width: 2px;
  border-radius: 3px;
  padding: 0 4px;
  font-family: var(--font-mono);
  font-size: 10px;
  color: var(--ink-mute);
}
.mono { font-family: var(--font-mono); font-variant-numeric: tabular-nums; }
.stamp {
  display: inline-block;
  border: 1px solid var(--rule);
  border-radius: 3px;
  padding: 1px 6px;
  font-family: var(--font-mono);
  font-size: 9.5px;
  letter-spacing: 0.06em;
  color: var(--ink-mute);
  white-space: nowrap;
}
.stamp.pending { color: var(--stamp); border-color: color-mix(in oklab, var(--stamp) 45%, transparent); }
.stamp.auto { color: var(--credit); border-color: color-mix(in oklab, var(--credit) 40%, transparent); }
.stamp.unparsed { color: var(--debit); border-color: color-mix(in oklab, var(--debit) 40%, transparent); }
</style>
