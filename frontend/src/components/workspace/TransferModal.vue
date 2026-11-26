<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppInput from '@/components/AppInput.vue';
import AppSelect from '@/components/AppSelect.vue';
import { transfers } from '@/lib/api/client';
import { formatCurrency, formatDate } from '@/lib/utils/format';
import { Search, Link2, Plus, ArrowLeftRight } from '@lucide/vue';

const props = defineProps<{ txns: any[]; accounts: any[]; prefill?: { sourceId: string; fromId: string } }>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'done'): void }>();

const fromId = ref(props.prefill?.fromId || '');
const toId = ref('');
const sourceId = ref(props.prefill?.sourceId || '');
const search = ref('');
const error = ref('');
const submitting = ref('');

const accName = (id: string) =>
  props.accounts.find((a) => a.id === id)?.nickname || props.accounts.find((a) => a.id === id)?.bankName || '';
const ms = (t: any) => (typeof t.occurredAt === 'number' ? t.occurredAt * 1000 : new Date(t.occurredAt || 0).getTime());
const DAY = 86400000;

watch(
  () => props.accounts,
  (accs) => {
    if (!accs.length) return;
    if (!fromId.value) fromId.value = accs[0].id;
    if (!toId.value) toId.value = accs.find((a) => a.id !== fromId.value)?.id || accs[0].id;
  },
  { immediate: true }
);

const sourceOptions = computed(() =>
  (props.txns || [])
    .filter((t) => t.accountId === fromId.value && t.type === 'DEBIT' && !t.linkedTransferId)
    .filter((t) => t.name.toLowerCase().includes(search.value.toLowerCase()))
    .sort((a, b) => ms(b) - ms(a))
);

const source = computed(() => (props.txns || []).find((t) => t.id === sourceId.value));

const candidates = computed(() => {
  if (!source.value || !toId.value || toId.value === fromId.value) return [];
  return (props.txns || [])
    .filter(
      (t) =>
        t.accountId === toId.value &&
        t.type === 'CREDIT' &&
        !t.linkedTransferId &&
        Math.abs(ms(t) - ms(source.value)) <= 5 * DAY &&
        Math.abs(t.amount - source.value.amount) <= source.value.amount * 0.1
    )
    .map((t) => ({
      ...t,
      diff: t.amount - source.value.amount,
      days: Math.round(Math.abs(ms(t) - ms(source.value)) / DAY)
    }))
    .sort((a, b) => Math.abs(a.diff) + a.days - (Math.abs(b.diff) + b.days));
});

async function link(cand: any) {
  submitting.value = 'link';
  error.value = '';
  try {
    await transfers().linkTransfers({ links: [{ debitTransactionId: source.value.id, creditTransactionId: cand.id }] });
    emit('done');
    emit('close');
  } catch (e: any) {
    error.value = e?.message || 'Link failed';
  } finally {
    submitting.value = '';
  }
}

async function createCounterpart() {
  if (!source.value || !toId.value) return;
  submitting.value = 'create';
  error.value = '';
  try {
    await transfers().createCounterpart({ transactionId: source.value.id, toAccountId: toId.value });
    emit('done');
    emit('close');
  } catch (e: any) {
    error.value = e?.message || 'Create failed';
  } finally {
    submitting.value = '';
  }
}
</script>

<template>
  <AppModal title="Transfer" @close="emit('close')">
    <div class="flex flex-col gap-4 px-5 py-4">
      <div class="grid grid-cols-2 gap-4">
        <AppSelect v-model="fromId" label="From account">
          <option v-for="a in accounts" :key="a.id" :value="a.id">{{ accName(a.id) }}</option>
        </AppSelect>
        <AppSelect v-model="toId" label="To account">
          <option v-for="a in accounts" :key="a.id" :value="a.id">{{ accName(a.id) }}</option>
        </AppSelect>
      </div>
      <div class="-my-1 flex items-center justify-center">
        <span class="flex h-7 w-7 items-center justify-center rounded-full border border-rule bg-page text-ink-mute">
          <ArrowLeftRight class="h-3.5 w-3.5" />
        </span>
      </div>

      <div v-if="!prefill?.sourceId">
        <AppInput v-model="search" label="Transaction" placeholder="Search unlinked debit transactions…">
          <template #icon>
            <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-ink-mute" stroke-width="2" />
          </template>
        </AppInput>
        <div class="mt-1.5 max-h-44 overflow-y-auto rounded-control border border-rule">
          <button
            v-for="t in sourceOptions"
            :key="t.id"
            type="button"
            class="w-full px-3 py-2 text-left text-[12.5px] text-ink transition-colors hover:bg-page-sunk"
            :class="t.id === sourceId ? 'bg-accent-wash' : ''"
            @click="sourceId = t.id"
          >
            <span class="font-medium">{{ t.name }}</span>
            <span class="text-ink-mute"> · {{ formatCurrency(t.amount) }} · {{ formatDate(t.occurredAt) }}</span>
          </button>
          <div v-if="!sourceOptions.length" class="px-3 py-3 text-[12px] text-ink-mute">
            No unlinked debit transactions in {{ accName(fromId) }}
          </div>
        </div>
      </div>
      <div v-else-if="source" class="rounded-control border border-rule bg-page-sunk/50 px-3 py-2.5">
        <div class="mb-0.5 text-label font-medium uppercase tracking-[0.1em] text-ink-mute">Source</div>
        <div class="text-[13px] text-ink font-medium">{{ source.name }}</div>
        <div class="text-[12px] text-ink-mute">
          {{ formatCurrency(source.amount) }} · {{ formatDate(source.occurredAt) }} · {{ accName(fromId) }}
        </div>
      </div>

      <div v-if="source && fromId !== toId">
        <span class="mb-1.5 block text-label font-medium uppercase tracking-[0.1em] text-ink-mute"
          >Matches in {{ accName(toId) }}</span
        >
        <div v-if="candidates.length" class="flex flex-col gap-1.5">
          <div
            v-for="c in candidates"
            :key="c.id"
            class="flex items-center justify-between gap-3 rounded-control border border-rule px-3 py-2"
          >
            <div class="min-w-0">
              <div class="text-[12.5px] text-ink truncate">{{ c.name }}</div>
              <div class="text-[11.5px] text-ink-mute">
                {{ formatCurrency(c.amount) }}
                <span v-if="c.diff !== 0">({{ c.diff > 0 ? '+' : '' }}{{ formatCurrency(c.diff) }})</span>
                <span> · {{ c.days }}d</span>
              </div>
            </div>
            <button type="button" class="ns-primary-xs shrink-0" :disabled="!!submitting" @click="link(c)">
              <Link2 class="h-3.5 w-3.5" />
              Link
            </button>
          </div>
        </div>
        <div v-else class="text-[12px] text-ink-mute">No matching transactions — create the counterpart below.</div>
      </div>

      <p v-if="error" class="text-[13px] text-debit">{{ error }}</p>

      <div class="flex items-center justify-end gap-2 border-t border-rule pt-3">
        <button type="button" class="ns-quiet" @click="emit('close')">Cancel</button>
        <button
          type="button"
          class="ns-primary"
          :disabled="!source || toId === fromId || !!submitting"
          @click="createCounterpart"
        >
          <Plus class="h-4 w-4" />
          {{ submitting === 'create' ? 'Creating…' : 'Create counterpart' }}
        </button>
      </div>
    </div>
  </AppModal>
</template>
