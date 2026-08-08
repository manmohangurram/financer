<script setup lang="ts">
import { ref } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppInput from '@/components/AppInput.vue';
import DatePicker from '@/components/DatePicker.vue';
import { dateToUnixSeconds, toLocalDateString } from '@/lib/utils/format';
import { roundMoney } from '@/lib/utils/money';

const props = defineProps<{ investment: any; side: 'buy' | 'sell' }>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'submit', payload: any): void }>();

const form = ref({
  quantity: undefined as number | undefined,
  price: undefined as number | undefined,
  occurredAt: toLocalDateString(new Date())
});

function handleSubmit() {
  if (!form.value.quantity || form.value.quantity <= 0 || form.value.price === undefined || form.value.price < 0) return;
  emit('submit', {
    investmentId: props.investment.id,
    side: props.side === 'buy' ? 1 : -1,
    quantity: roundMoney(form.value.quantity),
    price: roundMoney(form.value.price),
    occurredAt: { seconds: dateToUnixSeconds(form.value.occurredAt), nanos: 0 }
  });
}
</script>

<template>
  <AppModal :title="`${side === 'buy' ? 'Buy' : 'Sell'} — ${investment.name}`" @close="emit('close')">
    <form @submit.prevent="handleSubmit" class="p-6 space-y-4">
      <div v-if="side === 'sell'" class="rounded-xl border border-border bg-base-200 px-4 py-3 text-[13px] text-subtle">
        Current holding: <span class="font-semibold text-text">{{ investment.quantity }}</span> units
      </div>

      <AppInput
        v-model.number="form.quantity"
        label="Quantity"
        type="number"
        step="any"
        min="0"
        :placeholder="side === 'buy' ? 'e.g. 10' : 'e.g. 4'"
        required
      />

      <AppInput
        v-model.number="form.price"
        label="Price (₹)"
        type="number"
        step="any"
        min="0"
        placeholder="e.g. 2450.50"
        required
      />

      <div>
        <span class="block text-[12px] text-text-muted mb-1.5">Date</span>
        <DatePicker v-model="form.occurredAt" />
      </div>

      <div class="flex items-center justify-end gap-3 pt-2">
        <button type="button" @click="emit('close')" class="btn btn-ghost btn-sm border border-track">Cancel</button>
        <button type="submit" class="btn btn-success btn-sm">{{ side === 'buy' ? 'Add Buy' : 'Record Sell' }}</button>
      </div>
    </form>
  </AppModal>
</template>