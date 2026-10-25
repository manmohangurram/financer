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
    const [acc, msgs] = await Promise.all([mail().listAccounts({}), mail().listMessages({ status: filter.value })]);
    accounts.value = acc.accounts || [];
    messages.value = msgs.messages || [];
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

onMounted(load);
</script>

<template>
  <div class="w-full space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-[24px] font-bold text-base-content tracking-tight">Inbox</h1>
        <p class="text-[13.5px] text-subtle mt-0.5">Bank alerts waiting to become transactions</p>
      </div>
      <div class="join">
        <button
          v-for="f in FILTERS"
          :key="f"
          class="btn btn-sm join-item"
          :class="filter === f ? 'btn-primary' : 'btn-ghost border border-track'"
          @click="filter = f; load()"
        >
          {{ f }}
        </button>
      </div>
    </div>

    <div v-if="error" class="rounded-xl border border-warning/30 bg-warning/10 px-4 py-3 text-[13px] text-warning">{{ error }}</div>

    <div class="rounded-2xl border border-border bg-base-200">
      <button class="w-full flex items-center justify-between px-5 py-3.5" @click="accountsOpen = !accountsOpen">
        <span class="flex items-center gap-2 text-[14px] font-semibold text-text">
          <InboxIcon class="w-4 h-4" stroke-width="1.5" />
          Mailboxes
          <span class="text-faint font-normal">({{ accounts.length }})</span>
        </span>
        <ChevronDown class="w-4 h-4 text-subtle transition-transform" :class="accountsOpen ? 'rotate-180' : ''" />
      </button>
      <div v-if="accountsOpen" class="border-t border-border px-5 py-3 space-y-2">
        <p v-if="!accounts.length" class="text-[13px] text-subtle">
          No mailbox linked yet. Add one with an app password and the senders that send your bank alerts.
        </p>
        <div v-for="a in accounts" :key="a.id" class="flex items-center justify-between gap-3 text-[13px]">
          <span class="min-w-0">
            <span class="text-text font-medium">{{ a.label || a.username }}</span>
            <span class="text-faint"> · {{ a.username }}</span>
            <span v-if="a.lastError" class="text-expense"> · {{ a.lastError }}</span>
            <span v-else-if="a.lastPolledAt" class="text-faint"> · checked {{ formatDate(a.lastPolledAt) }}</span>
          </span>
          <span class="flex gap-1 shrink-0">
            <button class="btn btn-ghost btn-xs" :disabled="busy" @click="pollAccount(a.id)">
              <RefreshCw class="w-3.5 h-3.5" /> Check now
            </button>
            <button class="btn btn-ghost btn-xs text-expense" :disabled="busy" @click="removeAccount(a.id)">
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </span>
        </div>
        <button class="btn btn-ghost btn-xs gap-1 mt-1" @click="showMailboxForm = true">
          <Plus class="w-3.5 h-3.5" /> Add mailbox
        </button>
      </div>
    </div>

    <MailboxFormModal v-if="showMailboxForm" @close="showMailboxForm = false" @saved="load" />

    <div v-if="selected.size" class="sticky top-0 z-10 flex items-center justify-between gap-3 rounded-xl border border-primary-500/30 bg-primary-500/10 px-4 py-2.5">
      <span class="text-[13px] font-medium text-text">{{ selected.size }} selected</span>
      <span class="flex gap-2">
        <button class="btn btn-primary btn-sm" :disabled="busy" @click="act('APPROVE', [...selected])">Approve</button>
        <button class="btn btn-ghost btn-sm border border-track" :disabled="busy" @click="act('REJECT', [...selected])">Reject</button>
      </span>
    </div>

    <div v-if="loading" class="flex justify-center py-12">
      <span class="loading loading-dots loading-lg text-primary"></span>
    </div>

    <div v-else-if="!messages.length" class="rounded-2xl border border-border bg-base-200 px-8 py-14 text-center">
      <p class="text-[14px] text-subtle">Nothing here. Alerts are picked up on the next poll.</p>
    </div>

    <div v-else class="rounded-2xl border border-border bg-base-200 overflow-hidden">
      <div class="flex items-center gap-3 px-5 py-2.5 border-b border-border text-[12px] text-faint uppercase tracking-wide">
        <input
          type="checkbox"
          class="checkbox checkbox-xs"
          :checked="selected.size === actionable.length && actionable.length > 0"
          @change="toggleAll"
        />
        <span>From</span>
        <span class="ml-auto">Amount · merchant</span>
      </div>

      <template v-for="m in messages" :key="m.id">
        <div class="flex items-center gap-3 px-5 py-3 border-b border-border/60 last:border-0">
          <input
            v-if="m.status === 'PENDING' || m.status === 'UNPARSED'"
            type="checkbox"
            class="checkbox checkbox-xs"
            :checked="selected.has(m.id)"
            @change="toggle(m.id)"
          />
          <span v-else class="w-4"></span>

          <button class="flex-1 min-w-0 text-left" @click="expanded = expanded === m.id ? null : m.id">
            <span class="block text-[13.5px] font-medium text-text truncate">{{ m.fromAddr }}</span>
            <span class="block text-[12px] text-faint truncate">{{ m.subject || '(no subject)' }}</span>
          </button>

          <span class="text-right shrink-0">
            <span class="block text-[13.5px] font-semibold" :class="parsed(m)?.direction === 'CREDIT' ? 'text-income' : 'text-text'">
              {{ summary(m) }}
            </span>
            <span class="block text-[11.5px] text-faint">{{ hint(m) }}</span>
          </span>

          <span
            class="badge badge-sm shrink-0"
            :class="{
              'badge-warning': m.status === 'PENDING',
              'badge-ghost': m.status === 'UNPARSED',
              'badge-success': m.status === 'AUTO',
              'badge-error': m.status === 'REJECTED'
            }"
          >{{ m.status.toLowerCase() }}</span>

          <span v-if="m.status === 'PENDING' || m.status === 'UNPARSED'" class="flex gap-1 shrink-0">
            <button class="btn btn-primary btn-xs" :disabled="busy" @click="act('APPROVE', [m.id])">Approve</button>
            <button class="btn btn-ghost btn-xs border border-track" :disabled="busy" @click="act('REJECT', [m.id])">Reject</button>
          </span>
        </div>

        <div v-if="expanded === m.id" class="px-5 pb-4 space-y-3">
          <pre v-if="m.rawText" class="whitespace-pre-wrap rounded-xl bg-surface p-3 text-[12px] text-subtle max-h-64 overflow-auto">{{ m.rawText }}</pre>
          <p v-else class="text-[12px] text-faint">The message body was pruned once this row was resolved.</p>

          <div v-if="m.status === 'PENDING' || m.status === 'UNPARSED'" class="flex flex-wrap items-center gap-2">
            <span class="text-[12.5px] text-subtle">
              Account
              <span v-if="parsed(m)?.accountHint" class="text-faint">(mail says …{{ parsed(m)?.accountHint }})</span>
            </span>
            <select v-model="pickAccount[m.id]" class="select select-sm bg-surface border-border text-[13px]">
              <option value="">From the last 4 digits</option>
              <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.label || a.username }}</option>
            </select>
            <button class="btn btn-primary btn-xs" :disabled="busy" @click="act('APPROVE', [m.id])">Approve</button>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
