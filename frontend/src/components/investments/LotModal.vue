<script setup lang="ts">
import { ref } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppInput from '@/components/AppInput.vue';
import DatePicker from '@/components/DatePicker.vue';
import { dateToUnixSeconds, toLocalDateString } from '@/lib/utils/format';
import { roundMoney } from '@/lib/utils/money';

const props = defineProps<{ investment: any; side: 'buy' | 'sell'; lot?: any | null }>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'submit', payload: any): void; (e: 'update', payload: any): void; (e: 'delete'): void }>();

const form = ref({
  quantity: (props.lot?.quantity as number | undefined) ?? undefined,
  price: (props.lot?.price as number | undefined) ?? undefined,
  occurredAt: props.lot
    ? toLocalDateString(new Date(props.lot.occurredAt))
    : toLocalDateString(new Date())
});

function handleSubmit() {
  if (!form.value.quantity || form.value.quantity <= 0 || form.value.price === undefined || form.value.price < 0) return;
  const base = {
    investmentId: props.investment.id,
    side: (props.lot ? props.lot.side : props.side === 'buy' ? 1 : -1) as number,
    quantity: roundMoney(form.value.quantity),
    price: roundMoney(form.value.price),
    occurredAt: { seconds: dateToUnixSeconds(form.value.occurredAt), nanos: 0 }
  };
  if (props.lot) emit('update', { ...base, lotId: props.lot.id });
  else emit('submit', base);
}
</script>

<template>
  <AppModal :title="lot ? 'Edit Lot' : `${side === 'buy' ? 'Buy' : 'Sell'} — ${investment.name}`" @close="emit('close')">
    <form @submit.prevent="handleSubmit" class="p-6 space-y-4">
      <div v-if="!lot && side === 'sell'" class="rounded-xl border border-border bg-base-200 px-4 py-3 text-[13px] text-subtle">
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

      <div class="flex items-center justify-between pt-2">
        <button v-if="lot" type="button" class="btn btn-outline btn-error btn-sm" @click="emit('delete')">Delete</button>
        <span v-else></span>
        <div class="flex items-center gap-3">
          <button type="button" @click="emit('close')" class="btn btn-ghost btn-sm border border-track">Cancel</button>
          <button type="submit" class="btn btn-success btn-sm">{{ lot ? 'Update' : side === 'buy' ? 'Add Buy' : 'Record Sell' }}</button>
        </div>
      </div>
    </form>
  </AppModal>
</template>
