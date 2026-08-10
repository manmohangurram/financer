<script setup lang="ts">
import { ref } from 'vue';
import { SlidersHorizontal, X, Loader2, Eye } from '@lucide/vue';
import { rules } from '@/lib/api/client';
import AppInput from '@/components/AppInput.vue';
import AppSelect from '@/components/AppSelect.vue';
import TransactionTable from '@/components/workspace/TransactionTable.vue';
import { emptyOutput, type RuleOutput } from '@/lib/utils/ruleOutputs';

const props = defineProps<{ categories: any[]; accounts?: any[] }>();
const form = defineModel<{ name: string; priority: number; logic: number; conditions: { matchField: number; operator: number; pattern: string }[]; outputs: RuleOutput[] }>({ required: true });

const matchFields = [{ value: 1, label: 'Name' }, { value: 2, label: 'Amount' }, { value: 3, label: 'Type' }, { value: 4, label: 'Category' }, { value: 5, label: 'Account' }];
const operators = [{ value: 1, label: 'Contains' }, { value: 2, label: 'Starts With' }, { value: 3, label: 'Ends With' }, { value: 4, label: 'Equals' }, { value: 5, label: '>' }, { value: 6, label: '<' }, { value: 7, label: 'Regex' }];
const nameOps = [
  { value: 1, label: 'Rename to' },
  { value: 2, label: 'Add prefix' },
  { value: 3, label: 'Add suffix' }
];

const preview = ref<{ loading: boolean; error: string; matches: any[] }>({ loading: false, error: '', matches: [] });

function addOutput() {
  form.value.outputs.push(emptyOutput());
}
function removeOutput(i: number) {
  form.value.outputs.splice(i, 1);
}
function addCondition() {
  form.value.conditions.push({ matchField: 1, operator: 1, pattern: '' });
}
function removeCondition(i: number) {
  form.value.conditions.splice(i, 1);
}

async function runPreview() {
  preview.value = { loading: true, error: '', matches: [] };
  try {
    const resp = await rules().previewRule({
      logic: form.value.logic,
      conditions: form.value.conditions.map((c) => ({ matchField: c.matchField, operator: c.operator, pattern: c.pattern })),
      limit: 20
    });
    preview.value.matches = resp?.transactions || [];
  } catch (e: any) {
    preview.value.error = e?.message || 'Preview failed';
  }
  preview.value.loading = false;
}
</script>

<template>
  <div class="space-y-4">
    <div class="grid grid-cols-3 gap-3">
      <AppInput v-model="form.name" label="Name" placeholder="e.g. Food Purchases" />
      <AppInput v-model.number="form.priority" label="Priority" type="number" />
      <AppSelect v-model.number="form.logic" label="Match logic">
        <option :value="1">OR (any condition)</option>
        <option :value="2">AND (all conditions)</option>
      </AppSelect>
    </div>
    <p class="text-[11px] text-subtle -mt-2">Lower priority numbers run first; later rules override earlier ones.</p>

    <div>
      <span class="block text-[12px] text-text-muted mb-2">Conditions</span>
      <div v-for="(cond, i) in form.conditions" :key="i" class="grid grid-cols-[1fr_1fr_2fr_auto] gap-2 mb-2 items-end">
        <AppSelect v-model.number="cond.matchField">
          <option v-for="f in matchFields" :key="f.value" :value="f.value">{{ f.label }}</option>
        </AppSelect>
        <AppSelect v-model.number="cond.operator">
          <option v-for="o in operators" :key="o.value" :value="o.value">{{ o.label }}</option>
        </AppSelect>
        <AppInput v-model="cond.pattern" placeholder="Value" />
        <button type="button" aria-label="Remove condition" @click="removeCondition(i)" class="p-2 rounded-lg text-subtle hover:text-expense hover:bg-expense/10">
          <X class="w-4 h-4" />
        </button>
      </div>
      <button type="button" @click="addCondition" class="text-[12px] text-primary-400 hover:underline">+ Add condition</button>
    </div>

    <div class="pt-1 border-t border-border">
      <div class="flex items-center gap-2 mb-1">
        <SlidersHorizontal class="w-4 h-4 text-primary-400" stroke-width="1.5" />
        <span class="text-[13px] font-medium text-text">Output actions</span>
      </div>
      <p class="text-[11.5px] text-subtle mb-3">Applied in order to every matched transaction. Multiple outputs are allowed.</p>

      <div v-for="(out, i) in form.outputs" :key="i" class="grid grid-cols-[1fr_1.2fr_2.4fr_auto] gap-2 mb-2 items-end">
        <AppSelect v-model="out.type">
          <option value="name">Set name</option>
          <option value="category">Set category</option>
          <option value="transfer">Transfer to account</option>
        </AppSelect>
        <AppSelect v-if="out.type === 'name'" v-model.number="out.nameOp">
          <option v-for="o in nameOps" :key="o.value" :value="o.value">{{ o.label }}</option>
        </AppSelect>
        <span v-else class="text-[11px] text-subtle pb-3">{{ out.type === 'category' ? '→ category' : '→ account' }}</span>
        <AppInput v-if="out.type === 'name'" v-model="out.value" placeholder="e.g. Netflix Subscription" />
        <AppSelect v-else-if="out.type === 'category'" v-model="out.categoryId">
          <option value="">Pick a category</option>
          <option v-for="c in props.categories" :key="c.id" :value="c.id">{{ c.name }}</option>
        </AppSelect>
        <AppSelect v-else v-model="out.transferAccountId">
          <option value="">Pick an account</option>
          <option v-for="a in props.accounts || []" :key="a.id" :value="a.id">{{ a.nickname || a.bankName }}</option>
        </AppSelect>
        <button type="button" aria-label="Remove output" @click="removeOutput(i)" class="p-2 rounded-lg text-subtle hover:text-expense hover:bg-expense/10">
          <X class="w-4 h-4" />
        </button>
      </div>
      <button type="button" @click="addOutput" class="text-[12px] text-primary-400 hover:underline">+ Add output</button>
    </div>

    <div class="pt-1 border-t border-border">
      <div class="flex items-center justify-between mb-2">
        <span class="text-[13px] font-medium text-text">Preview</span>
        <button type="button" class="btn btn-primary btn-sm btn-square" aria-label="Preview" title="Preview" :disabled="preview.loading" @click="runPreview">
          <Loader2 v-if="preview.loading" class="w-3.5 h-3.5 animate-spin" />
          <Eye v-else class="w-3.5 h-3.5" />
        </button>
      </div>
      <p class="text-[11.5px] text-subtle mb-2">Shows up to 20 matching transactions this rule would apply to.</p>
      <p v-if="preview.error" class="text-[12px] text-expense">{{ preview.error }}</p>
      <div v-else class="rounded-xl border border-border bg-surface overflow-hidden">
        <TransactionTable
          :transactions="preview.matches"
          :categories="props.categories"
          readonly
          scrollable
          :empty-text="preview.loading ? 'Loading…' : 'No matching transactions'"
        />
      </div>
    </div>
  </div>
</template>
