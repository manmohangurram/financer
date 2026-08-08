<script setup lang="ts">
import { computed } from 'vue';
import { categoryColorMap } from '@/lib/utils/categoryColor';
import { Pencil, Trash2, Tag } from '@lucide/vue';
import { formatCurrency } from '@/lib/utils/format';

const props = defineProps<{ categories: any[]; txns: any[] }>();
const emit = defineEmits<{ (e: 'edit', category: any): void; (e: 'delete', category: any): void }>();
const colors = computed(() => categoryColorMap(props.categories.map((c: any) => c.name)));

const spend = computed(() => {
  const map = new Map<string, number>();
  const count = new Map<string, number>();
  for (const t of props.txns) {
    if (t.type === 1) continue;
    for (const id of t.categoryIds || []) {
      map.set(id, (map.get(id) || 0) + (t.amount || 0));
      count.set(id, (count.get(id) || 0) + 1);
    }
  }
  return { map, count };
});

const maxSpend = computed(() => Math.max(1, ...props.categories.map((c: any) => spend.value.map.get(c.id) || 0)));
const totalSpend = computed(() => props.categories.reduce((s, c: any) => s + (spend.value.map.get(c.id) || 0), 0));
</script>

<template>
  <div class="space-y-2">
    <div v-for="cat in categories" :key="cat.id" class="flex items-center gap-3 px-4 py-3 rounded-xl bg-surface border border-border group">
      <span class="w-3 h-3 rounded-full shrink-0" :style="{ background: colors[cat.name] }"></span>
      <div class="flex-1 min-w-0">
        <div class="flex items-baseline justify-between gap-2">
          <span class="text-[14px] text-text font-medium truncate">{{ cat.name }}</span>
          <span class="text-[13px] font-semibold whitespace-nowrap">{{ formatCurrency(spend.map.get(cat.id) || 0) }}</span>
        </div>
        <div class="mt-1.5 flex items-center gap-2">
          <div class="flex-1 h-1.5 rounded-full bg-track/50 overflow-hidden">
            <div
              class="h-full rounded-full transition-all duration-300"
              :style="{ width: ((spend.map.get(cat.id) || 0) / maxSpend) * 100 + '%', background: colors[cat.name] }"
            ></div>
          </div>
          <span class="text-[11px] text-subtle whitespace-nowrap w-10 text-right">
            {{ totalSpend ? Math.round(((spend.map.get(cat.id) || 0) / totalSpend) * 100) : 0 }}%
          </span>
          <span class="text-[11px] text-subtle whitespace-nowrap w-12 text-right">{{ spend.count.get(cat.id) || 0 }} txns</span>
        </div>
      </div>
      <div class="flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
        <button aria-label="Edit" @click="emit('edit', cat)" class="p-1.5 rounded-lg text-subtle hover:text-primary-400 hover:bg-primary-500/10">
          <Pencil class="w-3.5 h-3.5" />
        </button>
        <button aria-label="Delete" @click="emit('delete', cat)" class="p-1.5 rounded-lg text-subtle hover:text-expense hover:bg-expense/10">
          <Trash2 class="w-3.5 h-3.5" />
        </button>
      </div>
    </div>
    <div v-if="categories.length === 0" class="text-center py-16 text-subtle">
      <Tag class="w-8 h-8 mx-auto mb-2 text-faint" stroke-width="1.5" />
      No categories yet — add one to organize your spending
    </div>
  </div>
</template>