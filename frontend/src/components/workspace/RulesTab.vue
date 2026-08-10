<script setup lang="ts">
import { computed, ref } from 'vue';
import { ArrowRight, Loader2, Play, Sparkles } from '@lucide/vue';
import { rules } from '@/lib/api/client';

const props = defineProps<{ rules: any[]; categories: any[]; accounts?: any[] }>();
const emit = defineEmits<{ (e: 'edit', rule: any): void; (e: 'refresh'): void }>();

const matchFields: Record<string, string> = { NAME: 'Name', AMOUNT: 'Amount', TYPE: 'Type', CATEGORY: 'Category', ACCOUNT: 'Account' };
const operators: Record<string, string> = { CONTAINS: 'contains', STARTS_WITH: 'starts with', ENDS_WITH: 'ends with', EQUALS: 'equals', GREATER_THAN: 'is >', LESS_THAN: 'is <', REGEX: 'matches' };
const running = ref<string | null>(null);

const sorted = computed(() => [...props.rules].sort((a, b) => (a.priority || 0) - (b.priority || 0)));

function catName(id: string) {
  return props.categories.find((c: any) => c.id === id)?.name || '';
}
function accountName(id: string) {
  const a = (props.accounts || []).find((x: any) => x.id === id);
  return a ? (a.nickname || a.bankName) : 'account';
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
    if (a.setName) {
      const op = a.setNameOp === 'ADD_PREFIX' ? 'Add prefix' : a.setNameOp === 'ADD_SUFFIX' ? 'Add suffix' : 'Rename';
      lines.push(`${op} "${a.setName}"`);
    }
    if (a.setCategoryId) {
      lines.push(`Set category: ${catName(a.setCategoryId)}`);
    }
    if (a.setTransferAccountId) {
      lines.push(`Transfer to: ${accountName(a.setTransferAccountId)}`);
    }
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
  <div class="space-y-1.5">
    <div
      v-for="rule in sorted"
      :key="rule.id"
      class="flex items-center gap-4 px-4 py-3 rounded-xl bg-surface border border-border transition-colors hover:border-primary-500/40"
    >
      <button
        type="button"
        class="flex-1 flex items-center gap-4 min-w-0 text-left"
        @click="emit('edit', rule)"
      >
        <span class="w-8 h-8 rounded-lg bg-primary-500/12 text-primary-400 flex items-center justify-center text-[12px] font-bold shrink-0">
          {{ rule.priority || '—' }}
        </span>

        <span class="flex-1 min-w-0">
          <span class="text-[13.5px] text-text font-medium truncate block">{{ rule.name }}</span>
        </span>

        <span class="flex-1 min-w-0">
          <span class="badge badge-outline text-[9.5px] text-subtle">{{ rule.logic === 'AND' ? 'AND' : 'OR' }}</span>
        </span>

        <span class="flex-1 min-w-0 space-y-0.5">
          <span
            v-for="(line, i) in condLines(rule).slice(0, 2)"
            :key="'c' + i"
            class="block text-[11.5px] text-text-secondary truncate"
          >{{ line }}</span>
          <span v-if="condLines(rule).length > 2" class="block text-[11px] text-subtle">+{{ condLines(rule).length - 2 }} more conditions</span>
          <span v-if="condLines(rule).length === 0" class="block text-[11.5px] text-subtle">No conditions</span>
        </span>

        <span class="flex-1 min-w-0 space-y-0.5">
          <span
            v-for="(line, i) in actionLines(rule).slice(0, 2)"
            :key="'a' + i"
            class="flex items-center gap-1.5 text-[11.5px] text-subtle truncate"
          >
            <ArrowRight class="w-3 h-3 text-primary-400 shrink-0" stroke-width="2" />
            <span class="truncate">{{ line }}</span>
          </span>
          <span v-if="actionLines(rule).length > 2" class="block text-[11px] text-subtle">+{{ actionLines(rule).length - 2 }} more outputs</span>
          <span v-if="actionLines(rule).length === 0" class="block text-[11.5px] text-subtle">No outputs</span>
        </span>
      </button>

      <button
        type="button"
        class="btn btn-ghost btn-sm gap-1"
        :title="running === rule.id ? 'Running…' : 'Match existing transactions now'"
        :disabled="!!running"
        @click="runNow(rule)"
      >
        <Loader2 v-if="running === rule.id" class="w-3 h-3 animate-spin" stroke-width="2" />
        <Play v-else class="w-3 h-3" stroke-width="2" />
        {{ running === rule.id ? 'Running' : 'Run now' }}
      </button>
    </div>
    <div v-if="sorted.length === 0" class="text-center py-16 text-subtle">
      <Sparkles class="w-8 h-8 mx-auto mb-2 text-faint" stroke-width="1.5" />
      No rules yet — add one to auto-categorize transactions
    </div>
  </div>
</template>