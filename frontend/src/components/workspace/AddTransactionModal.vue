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

async function submitManual() {
  submitting.value = true;
  error.value = '';
  try {
    const payload = { ...txnForm.value, amount: roundMoney(txnForm.value.amount), occurredAt: { seconds: dateToUnixSeconds(txnForm.value.occurredAt), nanos: 0 } };
    await transactions().createTransactions({ transactions: [payload] });
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
      <form class="flex flex-col gap-4" @submit.prevent="submitManual">
        <TransactionForm v-model="txnForm" :accounts="accounts" :categories="categories" />
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
