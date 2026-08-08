<script setup lang="ts">
import { computed } from 'vue';
import { categoryColorMap } from '@/lib/utils/categoryColor';
import AppInput from '@/components/AppInput.vue';
import AppSelect from '@/components/AppSelect.vue';
import DatePicker from '@/components/DatePicker.vue';

const props = defineProps<{ accounts: any[]; categories: any[] }>();
const colors = computed(() => categoryColorMap(props.categories.map((c: any) => c.name)));
const form = defineModel<{ name: string; amount: number; type: number; accountId: string; occurredAt: string; categoryIds: string[] }>({ required: true });

function toggleCategory(id: string) {
  const i = form.value.categoryIds.indexOf(id);
  if (i >= 0) form.value.categoryIds.splice(i, 1);
  else form.value.categoryIds.push(id);
}
</script>

<template>
  <div class="grid grid-cols-2 gap-x-4">
    <div class="space-y-3">
      <AppInput v-model="form.name" label="Name" placeholder="e.g. Coffee Shop" />
      <AppInput v-model.number="form.amount" label="Amount" type="number" step="0.01" />
      <div>
        <span class="block text-[12px] text-text-muted mb-1.5">Type</span>
        <div class="flex gap-2">
          <label v-for="t in [{ v: 0, label: 'Debit' }, { v: 1, label: 'Credit' }]" :key="t.v" class="flex-1 cursor-pointer">
            <input type="radio" :value="t.v" :checked="form.type === t.v" @change="form.type = t.v" class="peer sr-only" />
            <span class="block w-full text-center px-3 h-10 leading-10 rounded-lg border cursor-pointer transition-colors text-[13px] border-border text-text-muted hover:text-text peer-checked:border-primary-500 peer-checked:bg-primary-500/10 peer-checked:text-primary-500">
              {{ t.label }}
            </span>
          </label>
        </div>
      </div>
      <div>
        <span class="block text-[12px] text-text-muted mb-1.5">Categories</span>
        <div class="flex flex-wrap gap-1.5">
          <button
            v-for="cat in categories"
            :key="cat.id"
            type="button"
            @click="toggleCategory(cat.id)"
            class="badge gap-1.5 px-3 py-2.5 h-auto text-[12px] transition-colors"
            :class="form.categoryIds.includes(cat.id) ? 'badge-primary' : 'badge-outline text-text-muted hover:text-text'"
            :style="form.categoryIds.includes(cat.id) ? { backgroundColor: colors[cat.name], borderColor: colors[cat.name], color: '#fff' } : {}"
          >
            <span class="w-2 h-2 rounded-full" :style="{ background: colors[cat.name] }"></span>
            {{ cat.name }}
          </button>
        </div>
      </div>
    </div>
    <div class="space-y-3">
      <AppSelect v-model="form.accountId" label="Account">
        <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.accountNickname || a.bankName }}</option>
      </AppSelect>
      <div>
        <span class="block text-[12px] text-text-muted mb-1.5">Date</span>
        <DatePicker v-model="form.occurredAt" />
      </div>
    </div>
  </div>
</template>
