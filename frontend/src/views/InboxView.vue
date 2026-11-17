<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { accounts as accountsApi, categories, mail } from '@/lib/api/client';
import { parseExtraction, type Extraction, type MailMessage, type MailMessageDetail } from '@/lib/types/mail';
import MailboxFormModal from '@/components/mail/MailboxFormModal.vue';
import MailQueue from '@/components/mail/MailQueue.vue';
import EmailReader from '@/components/mail/EmailReader.vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import AppModal from '@/components/AppModal.vue';
import AppInput from '@/components/AppInput.vue';
import { Plus, RefreshCw, Settings2 } from '@lucide/vue';

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
const mailboxes = ref<MailAccount[]>([]);
const accounts = ref<{ id: string; nickname?: string; bankName?: string; endingNumbers?: string | null }[]>([]);
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
const PAGE_SIZE = 20;
const aiStatus = ref<any>({ running: false, totalSenders: 0, doneSenders: 0, currentSender: '' });
const accountPrompt = ref<string | null>(null);
const newAccount = ref({ bankName: '', nickname: '' });
const creatingAccount = ref(false);
let aiTimer: number | null = null;
let refreshTimer: number | null = null;

const total = ref(0);
const pageToken = ref('');
const nextToken = ref('');
const history = ref<string[]>([]);
const pageLabel = computed(() => {
  if (!total.value) return '0';
  return `${messages.value.length} of ${total.value} · page ${history.value.length + 1}`;
});

function changePage(delta: number) {
  if (delta < 0) {
    if (!history.value.length) return;
    pageToken.value = history.value[history.value.length - 1];
    history.value = history.value.slice(0, -1);
  } else {
    if (!nextToken.value) return;
    history.value = [...history.value, pageToken.value];
    pageToken.value = nextToken.value;
  }
  selectedId.value = '';
  detail.value = null;
  load();
}

function statusParam(): string {
  if (filter.value === 'review') return 'review';
  if (filter.value === 'unparsed') return 'unparsed';
  if (filter.value === 'auto') return 'auto';
  return 'all';
}

function resetPaging() {
  history.value = [];
  pageToken.value = '';
}

watch([filter, activeAccount], () => {
  resetPaging();
  load();
});

let searchTimer: number | null = null;
watch(search, () => {
  if (searchTimer) window.clearTimeout(searchTimer);
  searchTimer = window.setTimeout(() => {
    resetPaging();
    load();
  }, 300);
});

const selected = computed(() => messages.value.find((m) => m.id === selectedId.value) || null);
const parsed = computed<Extraction | null>(() => parseExtraction(selected.value?.parsedJson));
const actionable = computed(() => {
  if (!selected.value) return false;
  if (['PENDING', 'UNPARSED'].includes(selected.value.status)) return true;
  return selected.value.status === 'AUTO' && !selected.value.transactionId;
});
const accountOptions = computed(() =>
  accounts.value.map((a) => ({
    value: a.id,
    label: a.nickname || a.bankName || 'Account',
    hint: a.endingNumbers ? '···' + a.endingNumbers : ''
  }))
);

function mailboxName(id: string): string {
  const a = mailboxes.value.find((x) => x.id === id);
  return a ? a.label || a.username : 'Mailbox';
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

let loadSeq = 0;

async function loadMeta() {
  try {
    const [acc, catRes, finAccounts] = await Promise.all([
      mail().listAccounts({}),
      categories().listCategories({ pageSize: 200 }),
      accountsApi().listAccounts({})
    ]);
    mailboxes.value = acc.accounts || [];
    cats.value = catRes.categories || [];
    accounts.value = finAccounts.accounts || [];
  } catch (e: any) {
    error.value = e?.message || 'Could not load mailboxes';
  }
}

async function load(opts: { silent?: boolean } = {}) {
  const seq = ++loadSeq;
  if (!opts.silent) loading.value = true;
  error.value = '';
  try {
    const msgs = await mail().listMessages({
      status: statusParam(),
      limit: PAGE_SIZE,
      pageToken: pageToken.value || undefined,
      accountId: activeAccount.value !== 'all' ? activeAccount.value : undefined,
      search: search.value.trim() || undefined
    });
    if (seq !== loadSeq) return;
    messages.value = msgs.messages || [];
    total.value = msgs.total || 0;
    nextToken.value = msgs.nextPageToken || '';
    if (selectedId.value && !messages.value.some((m) => m.id === selectedId.value)) {
      selectedId.value = '';
      detail.value = null;
    }
    if (!selectedId.value) {
      const first = messages.value[0];
      if (first) await select(first);
    }
  } catch (e: any) {
    if (seq === loadSeq) error.value = e?.message || 'Failed to load the inbox';
  } finally {
    if (seq === loadSeq) loading.value = false;
  }
}

async function createAccountAndApprove() {
  const hint = accountPrompt.value;
  if (!hint) return;
  creatingAccount.value = true;
  error.value = '';
  try {
    const created = await accountsApi().createAccount({
      bankName: newAccount.value.bankName.trim() || `Account ···${hint}`,
      nickname: newAccount.value.nickname.trim(),
      type: 'SAVINGS',
      endingNumbers: hint
    });
    accountPrompt.value = null;
    await loadMeta();
    form.value.accountId = created?.id || accounts.value.find((a) => a.endingNumbers === hint)?.id || '';
    await approve();
  } catch (e: any) {
    error.value = e?.message || 'Could not create the account';
  }
  creatingAccount.value = false;
}

async function pollFull() {
  error.value = '';
  notice.value = 'Full poll started — scanning the whole mailbox.';
  try {
    await mail().pollFull({});
    startRefresh();
  } catch (e: any) {
    error.value = e?.message || 'Could not start the full poll';
  }
}

async function startAiParse() {
  if (aiStatus.value.running) return;
  error.value = '';
  notice.value = 'AI parse started.';
  try {
    aiStatus.value = await mail().aiParse({});
    watchAi();
  } catch (e: any) {
    error.value = e?.message || 'Could not start AI parsing';
  }
}

function watchAi() {
  stopAi();
  aiTimer = window.setInterval(async () => {
    try {
      const st = await mail().aiParseStatus({});
      aiStatus.value = st;
      if (!st.running) {
        stopAi();
        notice.value = 'AI parse finished.';
        await load({ silent: true });
        await loadMeta();
      }
    } catch {
      /* keep the banner until the next tick succeeds */
    }
  }, 2000);
}

function stopAi() {
  if (aiTimer) window.clearInterval(aiTimer);
  aiTimer = null;
}

function startRefresh() {
  if (refreshTimer) window.clearInterval(refreshTimer);
  let ticks = 0;
  refreshTimer = window.setInterval(async () => {
    ticks += 1;
    await load({ silent: true });
    if (ticks >= 5 || !checking.value) {
      if (refreshTimer) window.clearInterval(refreshTimer);
      refreshTimer = null;
      checking.value = false;
      await loadMeta();
    }
  }, 3000);
}

async function poll(force = false) {
  checking.value = true;
  error.value = '';
  try {
    const res = await mail().pollAll(force ? { force: 1 } : {});
    if (res?.status === 'throttled') {
      if (force) notice.value = 'Mail was checked recently.';
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
  if (!selected.value || aiStatus.value.running) return;
  // A deleted/new account: resolve the last-4 hint, else ask to create it.
  if (!form.value.accountId && parsed.value?.accountHint) {
    const hint = parsed.value.accountHint;
    const match = accounts.value.find((a) => (a.endingNumbers || '') === hint);
    if (match) {
      form.value.accountId = match.id;
    } else {
      accountPrompt.value = hint;
      formErrors.value = [];
      return;
    }
  }
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
      resetPaging();
      await load();
    }
  } catch (e: any) {
    error.value = e?.message || 'Could not approve this mail';
  }
  busy.value = false;
}

async function reject() {
  if (!selected.value || aiStatus.value.running) return;
  busy.value = true;
  error.value = '';
  try {
    const res = await mail().act({ messages: [{ id: selected.value.id, action: 'REJECT' }] });
    if (res.failed?.length) error.value = res.failed[0].error;
    else {
      notice.value = 'Mail rejected.';
      selectedId.value = '';
      detail.value = null;
      resetPaging();
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
  await loadMeta();
  await load();
  poll(false);
  try {
    const st = await mail().aiParseStatus({});
    aiStatus.value = st;
    if (st?.running) watchAi();
  } catch {
    /* status is best-effort */
  }
});
onUnmounted(() => {
  if (refreshTimer) window.clearInterval(refreshTimer);
  if (searchTimer) window.clearTimeout(searchTimer);
  stopAi();
});
</script>

<template>
  <div class="flex min-h-0 w-full flex-1 flex-col gap-3">
    <SectionHeader title="Inbox" subtitle="Read the alert, correct the fields, book it">
      <template #actions>
        <div class="flex flex-wrap items-center gap-2">
          <button class="btn btn-sm btn-outline" :disabled="checking" @click="poll(true)">
            <RefreshCw class="h-4 w-4" :class="checking ? 'animate-spin' : ''" aria-hidden="true" />
            Poll now
          </button>
          <button class="btn btn-sm btn-outline" :disabled="checking" @click="pollFull">
            Full poll
          </button>
          <button class="btn btn-sm btn-outline" :disabled="aiStatus.running" @click="startAiParse">
            {{ aiStatus.running ? 'AI parsing…' : 'Parse with AI' }}
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

    <div v-if="aiStatus.running" role="status" class="rounded-card border border-accent/40 bg-accent-wash px-3.5 py-2 text-[12.5px] text-ink">
      AI parsing {{ aiStatus.doneSenders }}/{{ aiStatus.totalSenders }} senders
      <span v-if="aiStatus.currentSender" class="text-ink-mute">· {{ aiStatus.currentSender }}</span>
      <span class="text-ink-mute"> — approve/reject is paused until it finishes.</span>
    </div>

    <div class="grid min-h-0 flex-1 overflow-hidden rounded-card border border-rule bg-page lg:grid-cols-[minmax(320px,400px)_minmax(0,1fr)]">
      <!-- Queue -->
      <MailQueue
        v-model:filter="filter"
        v-model:active-account="activeAccount"
        v-model:search="search"
        :messages="messages"
        :mailboxes="mailboxes"
        :loading="loading"
        :page-number="history.length + 1"
        :page-label="pageLabel"
        :selected-id="selectedId"
        :has-prev="history.length > 0"
        :has-next="!!nextToken"
        @select="select"
        @page="changePage"
        @add-mailbox="showMailboxForm = true"
      />


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
                  <option v-for="a in accountOptions" :key="a.value" :value="a.value">{{ a.label }}{{ a.hint ? ` ${a.hint}` : '' }}</option>
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
              <button type="submit" class="btn btn-primary" :disabled="busy || !actionable || aiStatus.running">Approve</button>
              <button type="button" class="btn btn-outline" :disabled="busy || !actionable || aiStatus.running" @click="reject">Reject</button>
              <span v-if="!actionable" class="ml-auto text-[11px] text-ink-faint">Already decided</span>
            </div>
          </div>
        </form>
      </section>
    </div>

    <MailboxFormModal v-if="showMailboxForm" :account="null" @close="showMailboxForm = false" @saved="load(); poll(false)" />

    <AppModal v-if="accountPrompt" title="Create the account" @close="accountPrompt = null">
      <form class="flex flex-col gap-4 px-5 py-4" @submit.prevent="createAccountAndApprove">
        <p class="text-[12.5px] text-ink-mute">
          No account ends with <span class="mono text-ink">···{{ accountPrompt }}</span>. Create it to book this
          transaction; the mail is linked automatically.
        </p>
        <AppInput v-model="newAccount.bankName" label="Bank" placeholder="e.g. HDFC Bank" required />
        <AppInput v-model="newAccount.nickname" label="Nickname" placeholder="e.g. Everyday" hint="Optional" />
        <div class="flex justify-end gap-2 border-t border-rule pt-3">
          <button type="button" class="btn btn-sm btn-ghost" @click="accountPrompt = null">Cancel</button>
          <button type="submit" class="btn btn-sm btn-primary" :disabled="creatingAccount">
            {{ creatingAccount ? 'Creating…' : 'Create and approve' }}
          </button>
        </div>
      </form>
    </AppModal>
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