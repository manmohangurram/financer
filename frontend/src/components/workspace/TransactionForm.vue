<script setup lang="ts">
import AppInput from '@/components/AppInput.vue';
import AppSelect from '@/components/AppSelect.vue';
import DatePicker from '@/components/DatePicker.vue';

defineProps<{ accounts: any[]; categories: any[] }>();
const form = defineModel<{ name: string; amount: number; type: 'DEBIT' | 'CREDIT'; accountId: string; occurredAt: string; categoryIds: string[]; refNo?: string }>({ required: true });

function toggleCategory(id: string) {
  const i = form.value.categoryIds.indexOf(id);
  if (i >= 0) form.value.categoryIds.splice(i, 1);
  else form.value.categoryIds.push(id);
}
</script>

<template>
  <div class="grid gap-4 md:grid-cols-2">
    <div class="flex flex-col gap-3">
      <AppInput v-model="form.name" label="Name" placeholder="e.g. Coffee Shop" />
      <AppInput v-model.number="form.amount" label="Amount" type="number" step="0.01" />
      <div>
        <span class="mb-1.5 block text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Type</span>
        <div class="flex gap-2">
          <label v-for="t in [{ v: 'DEBIT' as const, label: 'Debit' }, { v: 'CREDIT' as const, label: 'Credit' }]" :key="t.v" class="flex-1 cursor-pointer">
            <input type="radio" :value="t.v" :checked="form.type === t.v" @change="form.type = t.v" class="peer sr-only" />
            <span class="block h-[34px] w-full cursor-pointer rounded-control border border-rule text-center text-[12.5px] leading-[34px] text-ink-soft transition-colors hover:bg-page-sunk peer-checked:border-accent/50 peer-checked:bg-accent-wash peer-checked:text-accent">
              {{ t.label }}
            </span>
          </label>
        </div>
      </div>
      <div>
        <span class="mb-1.5 block text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Categories</span>
        <p v-if="!categories.length" class="text-[12px] text-ink-mute">No categories yet — entries stay uncategorised.</p>
        <div v-else class="flex flex-wrap gap-1.5">
          <button
            v-for="cat in categories"
            :key="cat.id"
            type="button"
            @click="toggleCategory(cat.id)"
            class="h-[28px] rounded-full border px-2.5 text-[12px] transition-colors"
            :class="form.categoryIds.includes(cat.id) ? 'border-accent/50 bg-accent-wash text-accent' : 'border-rule text-ink-soft hover:bg-page-sunk hover:text-ink'"
          >
            {{ cat.name }}
          </button>
        </div>
      </div>
    </div>
    <div class="flex flex-col gap-3">
      <AppSelect v-model="form.accountId" label="Account">
        <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.nickname || a.bankName }}</option>
      </AppSelect>
      <div>
        <span class="mb-1.5 block text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Date</span>
        <DatePicker v-model="form.occurredAt" />
      </div>
      <AppInput v-model="form.refNo" label="Reference no (optional)" placeholder="e.g. 24825922781DC" />
    </div>
  </div>
</template>
