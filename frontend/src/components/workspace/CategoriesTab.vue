<script setup lang="ts">
import { computed } from 'vue';
import { Pencil, Trash2 } from '@lucide/vue';
import { formatCurrency } from '@/lib/utils/format';
import AppEmpty from '@/components/ui/AppEmpty.vue';
import AppButton from '@/components/ui/AppButton.vue';

// Categories as ledger rows: name, share of the categories' spend, amount, count.
// One list, no per-category hue — the bar carries size, the numbers carry truth.
const props = defineProps<{ categories: any[]; txns: any[] }>();
const emit = defineEmits<{ (e: 'edit', category: any): void; (e: 'delete', category: any): void }>();

const spend = computed(() => {
  const map = new Map<string, number>();
  const count = new Map<string, number>();
  for (const t of props.txns) {
    if (t.type === 'CREDIT') continue;
    for (const id of t.categoryIds || []) {
      map.set(id, (map.get(id) || 0) + Math.abs(t.amount || 0));
      count.set(id, (count.get(id) || 0) + 1);
    }
  }
  return { map, count };
});

const rows = computed(() =>
  [...props.categories]
    .map((c) => ({ id: c.id, name: c.name, amount: spend.value.map.get(c.id) || 0, txns: spend.value.count.get(c.id) || 0 }))
    .sort((a, b) => b.amount - a.amount)
);
const max = computed(() => Math.max(1, ...rows.value.map((r) => r.amount)));
const total = computed(() => rows.value.reduce((s, r) => s + r.amount, 0));
</script>

<template>
  <div v-if="!rows.length" class="px-4 py-10">
    <AppEmpty title="No categories yet." hint="Add one to organise your spending." />
  </div>

  <div v-else class="flex flex-col">
    <div class="flex items-center gap-3 border-b border-rule px-4 py-2 text-label font-medium uppercase tracking-[0.06em] text-ink-mute">
      <span class="flex-1">Category</span>
      <span class="w-[34%]">Share of spend</span>
      <span class="w-[92px] text-right">Spent</span>
      <span class="w-[52px] text-right">Entries</span>
      <span class="w-[64px]"></span>
    </div>
    <div
      v-for="row in rows"
      :key="row.id"
      class="group flex items-center gap-3 border-b border-rule-soft px-4 py-[9px] last:border-0 hover:bg-page-sunk/60"
    >
      <span class="min-w-0 flex-1 truncate text-[13px] text-ink">{{ row.name }}</span>
      <span class="relative block h-[8px] w-[34%] overflow-hidden rounded-[2px] bg-page-sunk">
        <span
          class="block h-full border-l-2 border-accent bg-accent/25"
          :style="{ width: `${Math.max(2, (row.amount / max) * 100)}%` }"
        />
      </span>
      <span class="tabular w-[92px] text-right text-[12.5px] font-medium text-ink">{{ formatCurrency(row.amount) }}</span>
      <span class="tabular w-[52px] text-right text-[12px] text-ink-mute">{{ row.txns }}</span>
      <span class="flex w-[64px] shrink-0 justify-end gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
        <AppButton variant="icon" size="sm" label="Edit category" @click="emit('edit', row)">
          <Pencil class="h-[14px] w-[14px]" stroke-width="1.7" />
        </AppButton>
        <AppButton variant="icon" size="sm" label="Delete category" @click="emit('delete', row)">
          <Trash2 class="h-[14px] w-[14px]" stroke-width="1.7" />
        </AppButton>
      </span>
    </div>
    <div class="flex items-center gap-3 border-t border-rule bg-page-sunk px-4 py-2 text-[12px] text-ink-mute">
      <span class="flex-1">{{ rows.length }} categories</span>
      <span class="w-[92px] text-right font-display text-[14px] text-ink">{{ formatCurrency(total) }}</span>
      <span class="w-[52px] text-right">{{ rows.reduce((s, r) => s + r.txns, 0) }}</span>
      <span class="w-[64px]"></span>
    </div>
  </div>
</template>
