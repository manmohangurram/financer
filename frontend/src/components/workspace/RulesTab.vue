<script setup lang="ts">
import { computed, ref } from 'vue';
import { ArrowRight, Loader2, Pencil, Play, Trash2 } from '@lucide/vue';
import { rules } from '@/lib/api/client';
import AppButton from '@/components/ui/AppButton.vue';
import AppEmpty from '@/components/ui/AppEmpty.vue';
import AppStamp from '@/components/ui/AppStamp.vue';

// Rules as ledger rows: the priority in the margin, then what it matches and
// what it does. Only the row that is running shows a spinner.
const props = defineProps<{ rules: any[]; categories: any[]; accounts?: any[] }>();
const emit = defineEmits<{
  (e: 'edit', rule: any): void;
  (e: 'delete', rule: any): void;
  (e: 'refresh'): void;
  (e: 'create'): void;
}>();

const matchFields: Record<string, string> = { NAME: 'Name', AMOUNT: 'Amount', TYPE: 'Type', CATEGORY: 'Category', ACCOUNT: 'Account' };
const operators: Record<string, string> = {
  CONTAINS: 'contains', STARTS_WITH: 'starts with', ENDS_WITH: 'ends with',
  EQUALS: 'equals', GREATER_THAN: 'is >', LESS_THAN: 'is <', REGEX: 'matches'
};
const running = ref<string | null>(null);

const sorted = computed(() => [...props.rules].sort((a, b) => (a.priority || 0) - (b.priority || 0)));

function catName(id: string) {
  return props.categories.find((c: any) => c.id === id)?.name || '';
}
function accountName(id: string) {
  const a = (props.accounts || []).find((x: any) => x.id === id);
  return a ? a.nickname || a.bankName : 'account';
}
function condLabel(c: any) {
  return `${matchFields[c.matchField] || '?'} ${operators[c.operator] || '?'} "${c.pattern || ''}"`;
}
function condLines(rule: any) {
  return (rule.conditions || []).map(condLabel);
}
function actionLines(rule: any) {
  const lines: string[] = [];
  for (const a of rule.actions || []) {
    if (a.setName) lines.push(`${a.setNameOp === 'ADD_PREFIX' ? 'Prefix' : a.setNameOp === 'ADD_SUFFIX' ? 'Suffix' : 'Rename'} "${a.setName}"`);
    if (a.setCategoryId) lines.push(`Category: ${catName(a.setCategoryId)}`);
    if (a.setTransferAccountId) lines.push(`Transfer: ${accountName(a.setTransferAccountId)}`);
  }
  return lines;
}

async function runNow(rule: any) {
  running.value = rule.id;
  try {
    await rules().runRule({ id: rule.id });
    emit('refresh');
  } finally {
    running.value = null;
  }
}
</script>

<template>
  <div v-if="!sorted.length" class="px-4 py-8">
    <AppEmpty title="No rules yet." hint="A rule renames, categorises or links entries as they are written.">
      <template #action><AppButton size="sm" @click="emit('create')">Create the first rule</AppButton></template>
    </AppEmpty>
  </div>

  <div v-else class="flex flex-col">
    <div class="flex items-center gap-3 border-b border-rule px-4 py-2 text-label font-medium uppercase tracking-[0.06em] text-ink-mute">
      <span class="w-8 text-center">#</span>
      <span class="flex-1">Rule</span>
      <span class="w-[28%]">Matches</span>
      <span class="w-[24%]">Then</span>
      <span class="w-[150px]"></span>
    </div>

    <div
      v-for="rule in sorted"
      :key="rule.id"
      class="group flex items-center gap-3 border-b border-rule-soft px-4 py-[9px] last:border-0 hover:bg-page-sunk/60"
    >
      <span class="flex h-6 w-8 shrink-0 items-center justify-center rounded-[3px] border border-rule bg-page-sunk text-[11px] font-medium text-ink-soft">
        {{ rule.priority || '—' }}
      </span>

      <button type="button" class="flex-1 min-w-0 text-left" @click="emit('edit', rule)">
        <span class="block truncate text-[13px] text-ink">{{ rule.name }}</span>
        <span v-if="(rule.conditions || []).length > 1" class="mt-0.5 block">
          <AppStamp>{{ rule.logic === 'ALL' ? 'AND' : 'OR' }}</AppStamp>
        </span>
      </button>

      <span class="w-[28%] min-w-0">
        <span v-for="(line, i) in condLines(rule).slice(0, 2)" :key="'c' + i" class="block truncate text-[11.5px] text-ink-soft">{{ line }}</span>
        <span v-if="condLines(rule).length > 2" class="block text-[11px] text-ink-mute">+{{ condLines(rule).length - 2 }} more</span>
        <span v-if="!condLines(rule).length" class="block text-[11.5px] text-ink-faint">No conditions</span>
      </span>

      <span class="w-[24%] min-w-0">
        <span v-for="(line, i) in actionLines(rule).slice(0, 2)" :key="'a' + i" class="flex items-center gap-1.5 truncate text-[11.5px] text-ink-soft">
          <ArrowRight class="h-3 w-3 shrink-0 text-accent" stroke-width="2" />
          <span class="truncate">{{ line }}</span>
        </span>
        <span v-if="actionLines(rule).length > 2" class="block text-[11px] text-ink-mute">+{{ actionLines(rule).length - 2 }} more</span>
        <span v-if="!actionLines(rule).length" class="block text-[11.5px] text-ink-faint">No outputs</span>
      </span>

      <span class="flex w-[150px] shrink-0 items-center justify-end gap-1">
        <AppButton
          variant="quiet"
          size="sm"
          :label="running === rule.id ? 'Running' : 'Match existing entries now'"
          :disabled="!!running"
          @click="runNow(rule)"
        >
          <Loader2 v-if="running === rule.id" class="h-3.5 w-3.5 animate-spin" stroke-width="2" />
          <Play v-else class="h-3.5 w-3.5" stroke-width="2" />
          {{ running === rule.id ? 'Running' : 'Run' }}
        </AppButton>
        <AppButton variant="icon" size="sm" label="Edit rule" @click="emit('edit', rule)">
          <Pencil class="h-[14px] w-[14px]" stroke-width="1.7" />
        </AppButton>
        <AppButton variant="icon" size="sm" label="Delete rule" @click="emit('delete', rule)">
          <Trash2 class="h-[14px] w-[14px]" stroke-width="1.7" />
        </AppButton>
      </span>
    </div>
  </div>
</template>
