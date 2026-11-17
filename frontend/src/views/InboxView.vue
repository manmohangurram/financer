<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { categories, mail } from '@/lib/api/client';
import { formatCurrency, formatDate } from '@/lib/utils/format';
import { parseExtraction, type Extraction, type MailMessage, type MailMessageDetail } from '@/lib/types/mail';
import MailboxFormModal from '@/components/mail/MailboxFormModal.vue';
import EmailReader from '@/components/mail/EmailReader.vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import { Inbox as InboxIcon, Plus, RefreshCw, Settings2 } from '@lucide/vue';

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
const detail = ref<MailMessageDetail | null>(null);
const detailLoading = ref(false);
const showMailboxForm = ref(false);
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
    if (q && !`${m.subject} ${m.fromAddr} ${m.fromName}`.toLowerCase().includes(q)) return false;
    return true;
  });
});

const selected = computed(() => messages.value.find((m) => m.id === selectedId.value) || null);
const parsed = computed<Extraction | null>(() => parseExtraction(selected.value?.parsedJson));
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

function amountOf(m: MailMessage): string {
  const p = parseExtraction(m.parsedJson);
  if (!p?.amount) return '';
  const sign = p.direction === 'CREDIT' ? '+' : '−';
  return `${sign} ${formatCurrency(p.amount)}`;
}

function stampClass(status: string): string {
  if (status === 'AUTO') return 'badge-success';
  if (status === 'UNPARSED') return 'badge-error';
  if (status === 'PENDING') return 'badge-warning';
  return '';
}

function prepForm(m: MailMessage) {
  const p = parseExtraction(m.parsedJson);
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

async function select(m: MailMessage) {
  selectedId.value = m.id;
  detail.value = { ...m, bodyText: null, rawText: null };
  prepForm(m);
  error.value = '';
  notice.value = '';
  detailLoading.value = true;
  try {
    detail.value = await mail().getMessage({ id: m.id });
  } catch (e: any) {
    error.value = e?.message || 'Could not read that message';
  } finally {
    detailLoading.value = false;
  }
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
      detail.value = null;
    }
    if (!selectedId.value) {
      const first = filtered.value[0];
      if (first) await select(first);
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
      messages: [
        {
          id: selected.value.id,
          action: 'APPROVE',
          name: form.value.name.trim(),
          amount: Number(form.value.amount),
          type: form.value.type,
          occurredAt: form.value.date,
          accountId: form.value.accountId,
          categoryIds: form.value.categoryIds
        }
      ]
    });
    if (res.failed?.length) {
      error.value = res.failed[0].error;
    } else {
      notice.value = 'Transaction created.';
      selectedId.value = '';
      detail.value = null;
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
      detail.value = null;
      await load();
    }
  } catch (e: any) {
    error.value = e?.message || 'Could not reject this mail';
  }
  busy.value = false;
}

function toggleCategory(id: string) {
  if (form.value.categoryIds.includes(id)) {
    form.value.categoryIds = form.value.categoryIds.filter((x) => x !== id);
  } else {
    form.value.categoryIds.push(id);
  }
}

onMounted(async () => {
  await load();
  poll(false);
});
onUnmounted(() => {
  if (refreshTimer) window.clearInterval(refreshTimer);
});
</script>

<template>
  <div class="flex min-h-0 w-full flex-1 flex-col gap-3">
    <SectionHeader title="Inbox" subtitle="Read the alert, correct the fields, book it">
      <template #actions>
        <div class="flex items-center gap-2">
          <button class="btn btn-sm btn-outline" :disabled="checking" @click="poll(true)">
            <RefreshCw class="h-4 w-4" :class="checking ? 'animate-spin' : ''" aria-hidden="true" />
            Poll now
          </button>
          <button class="btn btn-sm btn-ghost" @click="router.push('/settings?tab=mailboxes')">
            <Settings2 class="h-4 w-4" aria-hidden="true" />
            Mailboxes
          </button>
          <button class="btn btn-sm btn-primary" @click="showMailboxForm = true">
            <Plus class="h-4 w-4" aria-hidden="true" />
            Add mailbox
          </button>
        </div>
      </template>
    </SectionHeader>

    <p v-if="error" role="alert" class="rounded-card border border-debit/40 px-3.5 py-2 text-[12.5px] text-debit">{{ error }}</p>
    <p v-if="notice" class="rounded-card border border-credit/40 px-3.5 py-2 text-[12.5px] text-credit">{{ notice }}</p>

    <div class="grid min-h-0 flex-1 overflow-hidden rounded-card border border-rule bg-page lg:grid-cols-[minmax(320px,400px)_minmax(0,1fr)]">
      <!-- Queue -->
      <section class="flex max-h-[52vh] min-h-0 flex-col border-b border-rule lg:max-h-none lg:border-b-0 lg:border-r" aria-label="Mail queue">
        <div class="border-b border-rule bg-paper px-2 pt-1">
          <div class="tabs tabs-border w-full overflow-x-auto" role="tablist" aria-label="Mailboxes">
            <button
              role="tab"
              class="tab whitespace-nowrap text-[12.5px]"
              :class="activeAccount === 'all' ? 'tab-active' : ''"
              @click="activeAccount = 'all'"
            >
              All mailboxes
            </button>
            <button
              v-for="a in accounts"
              :key="a.id"
              role="tab"
              class="tab whitespace-nowrap text-[12.5px]"
              :class="activeAccount === a.id ? 'tab-active' : ''"
              @click="activeAccount = a.id"
            >
              {{ a.label || a.username }}
              <span v-if="a.lastError" class="ml-1 inline-block h-1.5 w-1.5 rounded-full bg-debit" :title="a.lastError"></span>
            </button>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-2 border-b border-rule-soft px-3 py-2">
          <div class="tabs tabs-sm" role="tablist" aria-label="Queue filter">
            <button
              v-for="f in FILTERS"
              :key="f.key"
              role="tab"
              class="tab text-[12px]"
              :class="filter === f.key ? 'tab-active' : ''"
              @click="filter = f.key"
            >
              {{ f.label }}
            </button>
          </div>
          <input
            v-model="search"
            type="search"
            placeholder="Search sender or subject"
            aria-label="Search mail"
            class="input input-sm input-bordered ml-auto w-[200px] text-[12.5px]"
          />
        </div>

        <div class="inbox-scroll min-h-0 flex-1 overflow-y-auto">
          <div v-if="loading" class="py-16 text-center text-[13px] text-ink-mute">Loading…</div>
          <div v-else-if="!filtered.length" class="flex flex-col items-center gap-2 px-6 py-16 text-center">
            <InboxIcon class="h-8 w-8 text-ink-faint" aria-hidden="true" />
            <p class="text-[13px] text-ink-mute">Nothing to review here.</p>
            <p class="text-[12px] text-ink-faint">New mail appears automatically after a poll.</p>
          </div>
          <button
            v-for="m in filtered"
            :key="m.id"
            class="flex w-full gap-2.5 border-b border-rule-soft px-3.5 py-3 text-left transition-colors"
            :class="selectedId === m.id ? 'bg-page-sunk shadow-[inset_2px_0_0_var(--accent)]' : 'bg-page hover:bg-page-sunk/60'"
            :aria-current="selectedId === m.id ? 'true' : undefined"
            @click="select(m)"
          >
            <span
              class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full"
              :class="m.status === 'UNPARSED' ? 'bg-debit' : m.status === 'AUTO' ? 'bg-credit' : 'bg-stamp'"
              aria-hidden="true"
            ></span>
            <span class="min-w-0 flex-1">
              <span class="flex items-baseline justify-between gap-2">
                <span class="truncate text-[13px] font-medium text-ink">{{ m.fromName || m.fromAddr }}</span>
                <span class="mono shrink-0 text-[11.5px]" :class="parseExtraction(m.parsedJson)?.direction === 'CREDIT' ? 'text-credit' : 'text-debit'">
                  {{ amountOf(m) }}
                </span>
              </span>
              <span class="mt-0.5 block truncate text-[12.5px] text-ink-soft">{{ m.subject || '(no subject)' }}</span>
              <span class="mt-1 flex items-center justify-between gap-2 text-[11px] text-ink-mute">
                <span class="truncate">{{ m.bodyPreview || m.fromAddr }}</span>
                <span class="flex shrink-0 items-center gap-2">
                  <span>{{ m.receivedAt ? formatDate(m.receivedAt) : '—' }}</span>
                  <span class="badge badge-xs badge-outline" :class="stampClass(m.status)">{{ m.status }}</span>
                </span>
              </span>
            </span>
          </button>
        </div>

        <div class="hidden border-t border-rule-soft bg-paper px-3 py-1.5 text-[11px] text-ink-mute lg:block">
          {{ filtered.length }} in this view
        </div>
      </section>

      <!-- Detail: email on top, booking form below -->
      <section class="inbox-scroll min-h-0 overflow-y-auto" aria-label="Message and booking">
        <EmailReader :message="detail" :mailbox="detail ? mailboxName(detail.mailAccountId) : ''" :loading="detailLoading" />

        <form v-if="selected" class="border-t border-rule bg-page" @submit.prevent="approve">
          <div class="flex flex-wrap items-center justify-between gap-2 border-b border-rule-soft px-5 py-3 md:px-7">
            <div>
              <h3 class="text-[15px] font-semibold text-ink">Book this transaction</h3>
              <p class="text-[12px] text-ink-mute">Correct anything the parser missed, then approve.</p>
            </div>
            <span v-if="parsed?.accountHint" class="text-[11.5px] text-ink-mute">
              Account in mail <span class="mono text-ink-soft">{{ parsed.accountHint }}</span>
            </span>
          </div>

          <div class="px-5 py-4 md:px-7">
            <div v-if="formErrors.length" role="alert" class="mb-4 rounded-control border border-debit/40 bg-debit/5 px-3 py-2 text-[12px] text-debit">
              <b class="mb-0.5 block">There is a problem</b>
              <ul class="list-inside list-disc">
                <li v-for="e in formErrors" :key="e">{{ e }}</li>
              </ul>
            </div>

            <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <label class="flex flex-col gap-1.5 md:col-span-2">
                <span class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Name</span>
                <input
                  v-model="form.name"
                  class="input input-bordered w-full text-[13.5px]"
                  :class="!form.name.trim() ? 'input-error' : ''"
                  placeholder="Merchant or payee"
                />
              </label>

              <label class="flex flex-col gap-1.5">
                <span class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Amount</span>
                <input
                  v-model="form.amount"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input input-bordered mono w-full text-[13.5px]"
                  :class="!form.amount ? 'input-error' : ''"
                />
              </label>

              <label class="flex flex-col gap-1.5">
                <span class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Type</span>
                <select v-model="form.type" class="select select-bordered w-full text-[13.5px]">
                  <option value="DEBIT">Debit</option>
                  <option value="CREDIT">Credit</option>
                </select>
              </label>

              <label class="flex flex-col gap-1.5">
                <span class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Date</span>
                <input
                  v-model="form.date"
                  type="date"
                  class="input input-bordered w-full text-[13.5px]"
                  :class="!form.date ? 'input-error' : ''"
                />
              </label>

              <label class="flex flex-col gap-1.5 md:col-span-2">
                <span class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Account</span>
                <select
                  v-model="form.accountId"
                  class="select select-bordered w-full text-[13.5px]"
                  :class="!form.accountId ? 'select-error' : ''"
                >
                  <option value="" disabled>Choose an account</option>
                  <option v-for="a in accountOptions" :key="a.value" :value="a.value">{{ a.label }}</option>
                </select>
              </label>

              <div class="flex flex-col gap-1.5 md:col-span-2 xl:col-span-4">
                <span class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Categories</span>
                <div class="flex flex-wrap gap-1.5">
                  <button
                    v-for="c in cats"
                    :key="c.id"
                    type="button"
                    class="btn btn-xs"
                    :class="form.categoryIds.includes(c.id) ? 'btn-primary' : 'btn-outline'"
                    :aria-pressed="form.categoryIds.includes(c.id)"
                    @click="toggleCategory(c.id)"
                  >
                    {{ c.name }}
                  </button>
                  <span v-if="!cats.length" class="text-[11.5px] text-ink-faint">No categories yet.</span>
                </div>
              </div>
            </div>

            <div class="mt-5 flex flex-wrap items-center gap-2 border-t border-rule-soft pt-4">
              <button type="submit" class="btn btn-primary" :disabled="busy || !actionable">Approve</button>
              <button type="button" class="btn btn-outline" :disabled="busy || !actionable" @click="reject">Reject</button>
              <span v-if="!actionable" class="ml-auto text-[11px] text-ink-faint">Already decided</span>
            </div>
          </div>
        </form>
      </section>
    </div>

    <MailboxFormModal v-if="showMailboxForm" :account="null" @close="showMailboxForm = false" @saved="load(); poll(false)" />
  </div>
</template>

<style scoped>
.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
/* The queue and the reader scroll independently, so both need a visible,
   tokenised scrollbar rather than an invisible overlay. */
.inbox-scroll {
  scrollbar-width: thin;
  scrollbar-color: var(--rule-strong) transparent;
  scrollbar-gutter: stable;
}
.inbox-scroll::-webkit-scrollbar {
  width: 10px;
}
.inbox-scroll::-webkit-scrollbar-track {
  background: transparent;
}
.inbox-scroll::-webkit-scrollbar-thumb {
  background: var(--rule-strong);
  border: 3px solid transparent;
  border-radius: 999px;
  background-clip: content-box;
}
.inbox-scroll::-webkit-scrollbar-thumb:hover {
  background: var(--ink-faint);
  background-clip: content-box;
}
</style>
