<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { RefreshCw, Trash2, Inbox as InboxIcon, ChevronDown, Plus } from '@lucide/vue';
import MailboxFormModal from '@/components/mail/MailboxFormModal.vue';
import { mail } from '@/lib/api/client';
import { formatCurrency, formatDate } from '@/lib/utils/format';

interface Parsed {
  amount?: number;
  direction?: 'DEBIT' | 'CREDIT';
  occurredAt?: string | null;
  merchant?: string;
  accountHint?: string | null;
  confidence?: number;
}

interface MailMessage {
  id: string;
  messageId: string;
  fromAddr: string;
  subject: string;
  receivedAt?: string | null;
  status: string;
  parsedJson?: string | null;
  confidence: number;
  rawText?: string | null;
  transactionId?: string | null;
}

interface MailAccount {
  id: string;
  label: string;
  username: string;
  lastPolledAt?: string | null;
  lastError: string;
}

const accounts = ref<MailAccount[]>([]);
const messages = ref<MailMessage[]>([]);
const accountsOpen = ref(false);
const loading = ref(true);
const busy = ref(false);
const error = ref('');
const selected = ref<Set<string>>(new Set());
const expanded = ref<string | null>(null);
const showMailboxForm = ref(false);
const pickAccount = ref<Record<string, string>>({});
const rules = ref<any[]>([]);
const rulesOpen = ref(false);
const newRule = ref({ fromPattern: '', subjectContains: '', action: 'REVIEW' });
const allMessages = ref<MailMessage[]>([]);

/// Senders that have earned an AUTO rule: several created, none rejected, and
/// no AUTO rule for them yet.
const suggestions = computed(() => {
  const stats: Record<string, { created: number; rejected: number }> = {};
  for (const m of allMessages.value) {
    const s = (stats[m.fromAddr] ||= { created: 0, rejected: 0 });
    if (m.status === 'AUTO') s.created += 1;
    if (m.status === 'REJECTED') s.rejected += 1;
  }
  const already = new Set(rules.value.filter((r) => r.action === 'AUTO').map((r) => String(r.fromPattern).toLowerCase()));
  return Object.entries(stats)
    .filter(([from, s]) => s.created >= 3 && s.rejected === 0 && !already.has(from.toLowerCase()))
    .map(([from, s]) => ({ from, created: s.created }));
});

const FILTERS = ['pending', 'unparsed', 'all'] as const;
const filter = ref<(typeof FILTERS)[number]>('pending');

function parsed(m: MailMessage): Parsed | null {
  if (!m.parsedJson) return null;
  try {
    return JSON.parse(m.parsedJson) as Parsed;
  } catch {
    return null;
  }
}

function summary(m: MailMessage): string {
  const p = parsed(m);
  if (!p?.amount) return m.subject || '(no subject)';
  const sign = p.direction === 'CREDIT' ? '+' : '-';
  return `${sign}${formatCurrency(p.amount)} · ${p.merchant || 'Unknown'}`;
}

function hint(m: MailMessage): string {
  const p = parsed(m);
  if (!p) return '';
  const parts: string[] = [];
  if (p.accountHint) parts.push(`acct …${p.accountHint}`);
  if (p.occurredAt) parts.push(formatDate(p.occurredAt));
  if (m.status === 'UNPARSED') parts.push('not parsed');
  return parts.join(' · ');
}

const actionable = computed(() => messages.value.filter((m) => m.status === 'PENDING' || m.status === 'UNPARSED'));

async function load() {
  loading.value = true;
  error.value = '';
  try {
    const [acc, msgs, rl, all] = await Promise.all([
      mail().listAccounts({}),
      mail().listMessages({ status: filter.value }),
      mail().listRules({}),
      mail().listMessages({ status: 'all', limit: 500 })
    ]);
    accounts.value = acc.accounts || [];
    messages.value = msgs.messages || [];
    rules.value = rl.rules || [];
    allMessages.value = all.messages || [];
    selected.value = new Set();
  } catch (e: any) {
    error.value = e?.message || 'Failed to load the inbox';
  }
  loading.value = false;
}

function toggle(id: string) {
  const next = new Set(selected.value);
  next.has(id) ? next.delete(id) : next.add(id);
  selected.value = next;
}

function toggleAll() {
  selected.value = selected.value.size === actionable.value.length ? new Set() : new Set(actionable.value.map((m) => m.id));
}

async function act(action: 'APPROVE' | 'REJECT', ids: string[], accountId?: string) {
  if (!ids.length) return;
  busy.value = true;
  error.value = '';
  try {
    const res = await mail().act({
      messages: ids.map((id) => ({ id, action, accountId: accountId || pickAccount.value[id] || undefined }))
    });
    if (res.failed?.length) {
      error.value = `${res.failed.length} could not be ${action === 'APPROVE' ? 'approved' : 'rejected'}: ${res.failed[0].error}`;
    }
    await load();
  } catch (e: any) {
    error.value = e?.message || 'Action failed';
  }
  busy.value = false;
}

async function pollAccount(id: string) {
  busy.value = true;
  error.value = '';
  try {
    const r = await mail().pollAccount({ id });
    if (r.stored === 0) error.value = `Checked ${r.matched} message(s); nothing new.`;
    await load();
  } catch (e: any) {
    error.value = e?.message || 'Poll failed';
  }
  busy.value = false;
}

async function removeAccount(id: string) {
  busy.value = true;
  try {
    await mail().deleteAccount({ id });
    await load();
  } catch (e: any) {
    error.value = e?.message || 'Could not remove the mailbox';
  }
  busy.value = false;
}

function ruleLabel(r: any): string {
  const parts: string[] = [];
  if (r.fromPattern) parts.push(`from ${r.fromPattern}`);
  if (r.subjectContains) parts.push(`subject has "${r.subjectContains}"`);
  if (r.bodyContains) parts.push(`body has "${r.bodyContains}"`);
  return parts.join(' · ') || '(no conditions)';
}

async function addRule() {
  const body: Record<string, unknown> = { action: newRule.value.action, position: rules.value.length };
  if (newRule.value.fromPattern.trim()) body.fromPattern = newRule.value.fromPattern.trim();
  if (newRule.value.subjectContains.trim()) body.subjectContains = newRule.value.subjectContains.trim();
  if (!body.fromPattern && !body.subjectContains) {
    error.value = 'A rule needs a sender or a subject match';
    return;
  }
  busy.value = true;
  try {
    await mail().createRule(body);
    newRule.value = { fromPattern: '', subjectContains: '', action: 'REVIEW' };
    await load();
  } catch (e: any) {
    error.value = e?.message || 'Could not create the rule';
  }
  busy.value = false;
}

async function createAutoRule(from: string) {
  busy.value = true;
  try {
    await mail().createRule({ fromPattern: from, action: 'AUTO', position: rules.value.length });
    await load();
  } catch (e: any) {
    error.value = e?.message || 'Could not create the rule';
  }
  busy.value = false;
}

async function deleteRule(id: string) {
  busy.value = true;
  try {
    await mail().deleteRule({ id });
    await load();
  } catch (e: any) {
    error.value = e?.message || 'Could not delete the rule';
  }
  busy.value = false;
}

onMounted(load);
</script>

<template>
  <div class="w-full space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-[24px] font-bold text-base-content tracking-tight">Inbox</h1>
        <p class="text-[13.5px] text-ink-mute mt-0.5">Bank alerts waiting to become transactions</p>
      </div>
      <div class="flex items-end gap-4 border-b border-rule">
        <button
          v-for="f in FILTERS"
          :key="f"
          type="button"
          class="relative -mb-px pb-2 text-[13px] font-medium capitalize transition-colors"
          :class="filter === f ? 'text-ink' : 'text-ink-mute hover:text-ink-soft'"
          @click="filter = f; load()"
        >
          {{ f }}
          <span v-if="filter === f" class="absolute inset-x-0 bottom-0 h-[2px] rounded-full bg-accent"></span>
        </button>
      </div>
    </div>

    <div v-if="error" class="rounded-card border border-stamp/40 px-3.5 py-2.5 text-[12.5px] text-ink">{{ error }}</div>

    <div class="overflow-hidden rounded-card border border-rule bg-page">
      <button class="flex w-full items-center justify-between px-4 py-2.5" @click="accountsOpen = !accountsOpen">
        <span class="flex items-center gap-2 text-[15px] font-semibold text-ink">
          <InboxIcon class="w-4 h-4" stroke-width="1.5" />
          Mailboxes
          <span class="font-normal text-ink-faint">({{ accounts.length }})</span>
        </span>
        <ChevronDown class="w-4 h-4 text-ink-mute transition-transform" :class="accountsOpen ? 'rotate-180' : ''" />
      </button>
      <div v-if="accountsOpen" class="flex flex-col gap-2 border-t border-rule px-4 py-3">
        <p v-if="!accounts.length" class="text-[12.5px] text-ink-mute">
          No mailbox linked yet. Add one with an app password and the senders that send your bank alerts.
        </p>
        <div v-for="a in accounts" :key="a.id" class="flex items-center justify-between gap-3 text-[13px]">
          <span class="min-w-0">
            <span class="text-ink font-medium">{{ a.label || a.username }}</span>
            <span class="text-ink-faint"> · {{ a.username }}</span>
            <span v-if="a.lastError" class="text-debit"> · {{ a.lastError }}</span>
            <span v-else-if="a.lastPolledAt" class="text-ink-faint"> · checked {{ formatDate(a.lastPolledAt) }}</span>
          </span>
          <span class="flex gap-1 shrink-0">
            <button class="ns-icon" :disabled="busy" @click="pollAccount(a.id)">
              <RefreshCw class="w-3.5 h-3.5" /> Check now
            </button>
            <button class="ns-icon text-debit" :disabled="busy" @click="removeAccount(a.id)">
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </span>
        </div>
        <button class="ns-icon gap-1 mt-1" @click="showMailboxForm = true">
          <Plus class="w-3.5 h-3.5" /> Add mailbox
        </button>
      </div>
    </div>

    <MailboxFormModal v-if="showMailboxForm" @close="showMailboxForm = false" @saved="load" />

    <div class="overflow-hidden rounded-card border border-rule bg-page">
      <button class="flex w-full items-center justify-between px-4 py-2.5" @click="rulesOpen = !rulesOpen">
        <span class="text-[14px] font-semibold text-ink">Rules <span class="text-ink-faint font-normal">({{ rules.length }})</span></span>
        <ChevronDown class="w-4 h-4 text-ink-mute transition-transform" :class="rulesOpen ? 'rotate-180' : ''" />
      </button>
      <div v-if="rulesOpen" class="border-t border-rule px-5 py-3 space-y-2">
        <p v-if="!rules.length" class="text-[12.5px] text-ink-mute">
          No rules. Everything is reviewed by hand until you add one.
        </p>
        <div v-for="r in rules" :key="r.id" class="flex items-center justify-between gap-3 text-[13px]">
          <span class="min-w-0">
            <span class="mr-2 align-middle" :class="r.action === 'AUTO' ? 'text-credit' : r.action === 'IGNORE' ? 'text-ink-mute' : 'text-stamp'">
              {{ r.action.toLowerCase() }}
            </span>
            <span class="text-ink-muted">{{ ruleLabel(r) }}</span>
          </span>
          <button class="ns-icon text-debit shrink-0" :disabled="busy" @click="deleteRule(r.id)">
            <Trash2 class="w-3.5 h-3.5" />
          </button>
        </div>

        <div v-for="sugg in suggestions" :key="sugg.from" class="flex items-center justify-between gap-3 rounded-card border border-success/30 bg-success/10 px-3 py-2">
          <span class="text-[12.5px] text-ink">
            Auto-create for <span class="font-medium">{{ sugg.from }}</span>?
            <span class="text-ink-mute">{{ sugg.created }} created, 0 rejected</span>
          </span>
          <button class="btn btn-success btn-xs shrink-0" :disabled="busy" @click="createAutoRule(sugg.from)">Create rule</button>
        </div>

        <div class="flex flex-wrap items-center gap-2 pt-2">
          <input v-model="newRule.fromPattern" placeholder="from (sender or domain)" class="input input-sm bg-page border-rule text-[13px] flex-1 min-w-[180px]" />
          <input v-model="newRule.subjectContains" placeholder="subject contains" class="input input-sm bg-page border-rule text-[13px] flex-1 min-w-[150px]" />
          <select v-model="newRule.action" class="select select-sm bg-page border-rule text-[13px]">
            <option value="REVIEW">review</option>
            <option value="IGNORE">ignore</option>
            <option value="AUTO">auto</option>
          </select>
          <button class="ns-quiet border border-rule gap-1" :disabled="busy" @click="addRule">
            <Plus class="w-3.5 h-3.5" /> Add
          </button>
        </div>
      </div>
    </div>

    <div v-if="selected.size" class="sticky top-0 z-10 flex items-center justify-between gap-3 rounded-card border border-primary-500/30 bg-accent/10 px-4 py-2.5">
      <span class="text-[13px] font-medium text-ink">{{ selected.size }} selected</span>
      <span class="flex gap-2">
        <button class="ns-primary" :disabled="busy" @click="act('APPROVE', [...selected])">Approve</button>
        <button class="ns-quiet border border-rule" :disabled="busy" @click="act('REJECT', [...selected])">Reject</button>
      </span>
    </div>

    <div v-if="loading" class="flex justify-center py-12">
      <span class="loading loading-dots loading-lg text-primary"></span>
    </div>

    <div v-else-if="!messages.length" class="rounded-card border border-rule bg-page px-6 py-10 text-center">
      <p class="text-[14px] text-ink-mute">Nothing here. Alerts are picked up on the next poll.</p>
    </div>

    <div v-else class="rounded-card border border-rule bg-page overflow-hidden">
      <div class="flex items-center gap-3 border-b border-rule bg-page-sunk px-4 py-2 text-label font-medium uppercase tracking-[0.06em] text-ink-mute">
        <input
          type="checkbox"
          class="h-3.5 w-3.5 accent-[var(--accent)]"
          :checked="selected.size === actionable.length && actionable.length > 0"
          aria-label="Select every pending message"
          @change="toggleAll"
        />
        <span class="flex-1">From</span>
        <span class="w-[132px] text-right">Amount</span>
        <span class="w-[74px] text-right">State</span>
        <span class="w-[132px]"></span>
      </div>

      <template v-for="m in messages" :key="m.id">
        <div class="flex items-center gap-3 border-b border-rule-soft px-4 py-[9px] last:border-0 hover:bg-page-sunk/60">
          <input
            v-if="m.status === 'PENDING' || m.status === 'UNPARSED'"
            type="checkbox"
            class="h-3.5 w-3.5 accent-[var(--accent)]"
            :checked="selected.has(m.id)"
            aria-label="Select message"
            @change="toggle(m.id)"
          />
          <span v-else class="w-3.5"></span>

          <button class="min-w-0 flex-1 text-left" @click="expanded = expanded === m.id ? null : m.id">
            <span class="block truncate text-[13px] text-ink">{{ m.fromAddr }}</span>
            <span class="block truncate text-[11.5px] text-ink-mute">{{ m.subject || '(no subject)' }}</span>
          </button>

          <span class="w-[132px] shrink-0 text-right">
            <span class="tabular block text-[13px] font-medium" :class="parsed(m)?.direction === 'CREDIT' ? 'text-credit' : 'text-ink'">
              {{ summary(m) }}
            </span>
            <span class="block truncate text-[11px] text-ink-mute">{{ hint(m) }}</span>
          </span>

          <span class="w-[74px] shrink-0 text-right">
            <AppStamp :tone="m.status === 'PENDING' ? 'pending' : m.status === 'UNPARSED' ? 'neutral' : m.status === 'AUTO' ? 'credit' : 'debit'">
              {{ m.status.toLowerCase() }}
            </AppStamp>
          </span>

          <span v-if="m.status === 'PENDING' || m.status === 'UNPARSED'" class="flex w-[132px] shrink-0 justify-end gap-1">
            <AppButton size="sm" :disabled="busy" @click="act('APPROVE', [m.id])">Approve</AppButton>
            <AppButton variant="outline" size="sm" :disabled="busy" @click="act('REJECT', [m.id])">Reject</AppButton>
          </span>
          <span v-else class="w-[132px] shrink-0"></span>
        </div>

        <div v-if="expanded === m.id" class="flex flex-col gap-3 border-b border-rule-soft bg-page-sunk/40 px-4 py-3">
          <pre v-if="m.rawText" class="max-h-56 overflow-auto whitespace-pre-wrap rounded-control border border-rule bg-page p-3 text-[11.5px] text-ink-mute">{{ m.rawText }}</pre>
          <p v-else class="text-[12px] text-ink-faint">The message body was pruned once this row was resolved.</p>

          <div v-if="m.status === 'PENDING' || m.status === 'UNPARSED'" class="flex flex-wrap items-center gap-2">
            <span class="text-[12.5px] text-ink-mute">
              Account
              <span v-if="parsed(m)?.accountHint" class="text-ink-faint">(mail says …{{ parsed(m)?.accountHint }})</span>
            </span>
            <select v-model="pickAccount[m.id]" class="h-[30px] rounded-control border border-rule bg-page px-2 text-[12.5px] text-ink">
              <option value="">From the last 4 digits</option>
              <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.label || a.username }}</option>
            </select>
            <AppButton size="sm" :disabled="busy" @click="act('APPROVE', [m.id])">Approve</AppButton>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
