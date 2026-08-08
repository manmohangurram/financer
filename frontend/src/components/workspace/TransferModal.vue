<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppInput from '@/components/AppInput.vue';
import AppSelect from '@/components/AppSelect.vue';
import { transfers } from '@/lib/api/client';
import { formatCurrency, formatDate } from '@/lib/utils/format';
import { Search, Link2, Plus, ArrowLeftRight } from '@lucide/vue';

const props = defineProps<{ txns: any[]; accounts: any[] }>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'done'): void }>();

const fromId = ref('');
const toId = ref('');
const sourceId = ref('');
const search = ref('');
const error = ref('');
const submitting = ref('');

const accName = (id: string) => props.accounts.find((a) => a.id === id)?.accountNickname || props.accounts.find((a) => a.id === id)?.bankName || '';
const ms = (t: any) => (typeof t.occurredAt === 'number' ? t.occurredAt * 1000 : new Date(t.occurredAt || 0).getTime());
const DAY = 86400000;

watch(
  () => props.accounts,
  (accs) => {
    if (accs.length && !fromId.value) {
      fromId.value = accs[0].id;
      toId.value = accs[1]?.id || accs[0].id;
    }
  },
  { immediate: true }
);

const sourceOptions = computed(() =>
  (props.txns || [])
    .filter((t) => t.accountId === fromId.value && t.type === 0 && !t.linkedTransferId)
    .filter((t) => t.name.toLowerCase().includes(search.value.toLowerCase()))
    .sort((a, b) => ms(b) - ms(a))
);

const source = computed(() => (props.txns || []).find((t) => t.id === sourceId.value));

const candidates = computed(() => {
  if (!source.value || !toId.value || toId.value === fromId.value) return [];
  return (props.txns || [])
    .filter(
      (t) =>
        t.accountId === toId.value && t.type === 1 && !t.linkedTransferId &&
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
    const resp = await transfers().linkTransfers({ links: [{ debitTransactionId: source.value.id, creditTransactionId: cand.id }] });
    if (resp?.success === false) { error.value = resp.message || 'Link failed'; return; }
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
    <div class="p-6 space-y-4">
      <div class="grid grid-cols-2 gap-4">
        <AppSelect v-model="fromId" label="From account">
          <option v-for="a in accounts" :key="a.id" :value="a.id">{{ accName(a.id) }}</option>
        </AppSelect>
        <AppSelect v-model="toId" label="To account">
          <option v-for="a in accounts" :key="a.id" :value="a.id">{{ accName(a.id) }}</option>
        </AppSelect>
      </div>
      <div class="flex items-center justify-center my-1">
        <ArrowLeftRight class="w-4 h-4 text-primary-400" />
      </div>

      <AppInput v-model="search" label="Transaction" placeholder="Search unlinked debit transactions…">
        <template #icon>
          <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-subtle" stroke-width="2" />
        </template>
      </AppInput>
        <div class="mt-1 max-h-44 overflow-y-auto rounded-lg border border-border">
          <button
            v-for="t in sourceOptions"
            :key="t.id"
            type="button"
            class="w-full text-left px-3 py-2 text-[12.5px] text-text hover:bg-primary-500/10 transition-colors"
            :class="t.id === sourceId ? 'bg-primary-500/15' : ''"
            @click="sourceId = t.id"
          >
            <span class="font-medium">{{ t.name }}</span>
            <span class="text-subtle"> · {{ formatCurrency(t.amount) }} · {{ formatDate(t.occurredAt) }}</span>
          </button>
          <div v-if="!sourceOptions.length" class="px-3 py-3 text-[12px] text-subtle">No unlinked debit transactions in {{ accName(fromId) }}</div>
        </div>

      <div v-if="source && fromId !== toId">
        <label class="block text-[12px] text-text-muted mb-1">Matches in {{ accName(toId) }}</label>
        <div v-if="candidates.length" class="space-y-1.5">
          <div v-for="c in candidates" :key="c.id" class="flex items-center justify-between gap-3 px-3 py-2 rounded-lg border border-border">
            <div class="min-w-0">
              <div class="text-[12.5px] text-text truncate">{{ c.name }}</div>
              <div class="text-[11.5px] text-subtle">
                {{ formatCurrency(c.amount) }}
                <span v-if="c.diff !== 0">({{ c.diff > 0 ? '+' : '' }}{{ formatCurrency(c.diff) }})</span>
                <span> · {{ c.days }}d</span>
              </div>
            </div>
            <button type="button" class="btn btn-sm btn-primary gap-1 shrink-0" :disabled="!!submitting" @click="link(c)">
              <Link2 class="w-3.5 h-3.5" />
              Link
            </button>
          </div>
        </div>
        <div v-else class="text-[12px] text-subtle">No matching transactions — create the counterpart below.</div>
      </div>

      <p v-if="error" class="text-[13px] text-expense">{{ error }}</p>

      <div class="flex justify-end pt-2">
        <button type="button" class="btn btn-primary gap-1.5" :disabled="!source || toId === fromId || !!submitting" @click="createCounterpart">
          <Plus class="w-4 h-4" />
          {{ submitting === 'create' ? 'Creating…' : 'Create counterpart' }}
        </button>
      </div>
    </div>
  </AppModal>
</template>
