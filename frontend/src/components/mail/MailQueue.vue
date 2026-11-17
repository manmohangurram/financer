<script setup lang="ts">
import { Inbox as InboxIcon } from '@lucide/vue';
import { formatDate } from '@/lib/utils/format';
import { amountLabel, stampClass } from '@/lib/utils/mail';
import { parseExtraction, type MailMessage } from '@/lib/types/mail';

interface MailAccount {
  id: string;
  label: string;
  username: string;
  lastError: string;
}

defineProps<{
  messages: MailMessage[];
  mailboxes: MailAccount[];
  loading: boolean;
  pageNumber: number;
  pageLabel: string;
  selectedId: string;
  hasPrev: boolean;
  hasNext: boolean;
}>();

const emit = defineEmits<{
  (e: 'select', message: MailMessage): void;
  (e: 'page', delta: number): void;
  (e: 'add-mailbox'): void;
}>();

const filter = defineModel<'review' | 'unparsed' | 'auto' | 'all'>('filter', { required: true });
const activeAccount = defineModel<string>('activeAccount', { required: true });
const search = defineModel<string>('search', { required: true });

const FILTERS = [
  { key: 'review', label: 'Needs review' },
  { key: 'unparsed', label: 'Unparsed' },
  { key: 'auto', label: 'Auto' },
  { key: 'all', label: 'All' }
] as const;

function sender(m: MailMessage): string {
  return m.fromName || m.fromAddr;
}

function direction(m: MailMessage): string {
  return parseExtraction(m.parsedJson)?.direction || '';
}
</script>

<template>
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
          v-for="a in mailboxes"
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
      <div v-else-if="!messages.length" class="flex flex-col items-center gap-2 px-6 py-16 text-center">
        <InboxIcon class="h-8 w-8 text-ink-faint" aria-hidden="true" />
        <template v-if="!mailboxes.length">
          <p class="text-[13px] text-ink-mute">No mailboxes yet.</p>
          <p class="text-[12px] text-ink-faint">Link one to start reading bank alerts.</p>
          <button class="btn btn-sm btn-primary mt-1" @click="emit('add-mailbox')">Add mailbox</button>
        </template>
        <template v-else>
          <p class="text-[13px] text-ink-mute">Nothing to review here.</p>
          <p class="text-[12px] text-ink-faint">New mail appears automatically after a poll.</p>
        </template>
      </div>
      <button
        v-for="m in messages"
        :key="m.id"
        class="flex w-full gap-2.5 border-b border-rule-soft px-3.5 py-3 text-left transition-colors"
        :class="selectedId === m.id ? 'bg-page-sunk shadow-[inset_2px_0_0_var(--accent)]' : 'bg-page hover:bg-page-sunk/60'"
        :aria-current="selectedId === m.id ? 'true' : undefined"
        @click="emit('select', m)"
      >
        <span
          class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full"
          :class="m.status === 'UNPARSED' ? 'bg-debit' : m.status === 'AUTO' ? 'bg-credit' : 'bg-stamp'"
          aria-hidden="true"
        ></span>
        <span class="min-w-0 flex-1">
          <span class="flex items-baseline justify-between gap-2">
            <span class="truncate text-[13px] font-medium text-ink">{{ sender(m) }}</span>
            <span class="mono shrink-0 text-[11.5px]" :class="direction(m) === 'CREDIT' ? 'text-credit' : 'text-debit'">
              {{ amountLabel(m) }}
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

    <div class="flex items-center justify-between gap-2 border-t border-rule-soft bg-paper px-3 py-1.5 text-[11px] text-ink-mute">
      <span>{{ pageLabel }}</span>
      <span class="flex items-center gap-1">
        <button class="btn btn-ghost btn-xs px-1.5" :disabled="!hasPrev" aria-label="Previous page" @click="emit('page', -1)">‹</button>
        <span class="tabular px-1">{{ pageNumber }}</span>
        <button class="btn btn-ghost btn-xs px-1.5" :disabled="!hasNext" aria-label="Next page" @click="emit('page', 1)">›</button>
      </span>
    </div>
  </section>
</template>

<style scoped>
.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
/* The queue scrolls independently, so it needs a visible, tokenised scrollbar. */
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
