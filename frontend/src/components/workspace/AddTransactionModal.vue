<script setup lang="ts">
import { ref } from 'vue';
import AppModal from '@/components/AppModal.vue';
import TransactionForm from '@/components/workspace/TransactionForm.vue';
import { transactions } from '@/lib/api/client';
import { dateToUnixSeconds, toLocalDateString } from '@/lib/utils/format';
import { roundMoney } from '@/lib/utils/money';

const props = defineProps<{ accounts: any[]; categories: any[]; defaultAccountId: string }>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'imported'): void }>();

const txnForm = ref<{
  name: string; amount: number; type: 'DEBIT' | 'CREDIT';
  accountId: string; occurredAt: string; categoryIds: string[];
}>({
  name: '', amount: 0, type: 'DEBIT',
  accountId: props.defaultAccountId,
  occurredAt: toLocalDateString(new Date()),
  categoryIds: []
});

const error = ref('');
const submitting = ref(false);
const checking = ref(false);
const duplicateWarning = ref(false);

function payload() {
  return {
    ...txnForm.value,
    amount: roundMoney(txnForm.value.amount),
    occurredAt: { seconds: dateToUnixSeconds(txnForm.value.occurredAt), nanos: 0 }
  };
}

async function submitManual(force = false) {
  error.value = '';
  duplicateWarning.value = false;
  const body = payload();

  if (!force) {
    checking.value = true;
    try {
      const out = await transactions().findDuplicates({ transactions: [body] });
      if (out?.duplicates?.[0]) {
        duplicateWarning.value = true;
        checking.value = false;
        return;
      }
    } catch {
      // A failed probe must not block adding a transaction.
    }
    checking.value = false;
  }

  submitting.value = true;
  try {
    await transactions().createTransactions({ transactions: [body] });
    emit('imported');
    emit('close');
  } catch (e: any) {
    error.value = e.message || 'Failed to add transaction';
  }
  submitting.value = false;
}
</script>

<template>
  <AppModal title="Add transaction" @close="emit('close')">
    <div class="px-5 py-4">
      <form class="flex flex-col gap-4" @submit.prevent="submitManual()">
        <TransactionForm v-model="txnForm" :accounts="accounts" :categories="categories" />
        <div v-if="duplicateWarning" role="alert" class="flex flex-col gap-2 rounded-control border border-stamp/40 bg-stamp/5 px-3 py-2.5">
          <p class="m-0 text-[12.5px] text-ink">
            A transaction with this account, date, type and amount already exists.
          </p>
          <div class="flex items-center gap-2">
            <button type="button" class="ns-primary" @click="submitManual(true)">Add anyway</button>
            <button type="button" class="ns-quiet" @click="duplicateWarning = false">Keep editing</button>
          </div>
        </div>
        <p v-if="error" class="text-[13px] text-debit">{{ error }}</p>
        <div class="flex items-center justify-end gap-2 pt-1">
          <button type="button" class="ns-quiet" @click="emit('close')">Cancel</button>
          <button type="submit" class="ns-primary" :disabled="submitting">
            {{ submitting ? 'Adding…' : 'Add transaction' }}
          </button>
        </div>
      </form>
    </div>
  </AppModal>
</template>
