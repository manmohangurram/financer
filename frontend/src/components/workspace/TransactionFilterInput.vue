<script setup lang="ts">
import { ref, computed } from 'vue';
import FilterTransactionsPopover from '@/components/workspace/FilterTransactionsPopover.vue';
import { emptyFilters, buildFilterBubbles, removeFilterBubble, type FilterBubble, type TransactionFilters } from '@/lib/utils/transactionFilters';
import { Filter, ChevronDown, X } from '@lucide/vue';

const props = defineProps<{ categories: any[] }>();
const filters = defineModel<TransactionFilters>('filters', { default: emptyFilters });

const text = ref('');
const showPopup = ref(false);
const inputEl = ref<HTMLInputElement | null>(null);

const bubbles = computed(() => buildFilterBubbles(filters.value, props.categories));

function addName() {
  const n = text.value.trim();
  if (!n) return;
  if (!filters.value.names.includes(n)) {
    filters.value = { ...filters.value, names: [...filters.value.names, n] };
  }
  text.value = '';
  inputEl.value?.focus();
}

function remove(bubble: FilterBubble) {
  filters.value = removeFilterBubble(filters.value, bubble);
}

function onBackspace() {
  if (text.value !== '') return;
  const all = bubbles.value;
  if (all.length) remove(all[all.length - 1]);
}

function onApply(f: TransactionFilters) {
  filters.value = f;
  showPopup.value = false;
}
</script>

<template>
  <div class="relative flex-1 min-w-[400px]">
    <div
      class="flex items-center gap-1.5 flex-wrap px-2.5 py-1.5 rounded-xl bg-surface border border-border transition-colors"
      @click="inputEl?.focus()"
    >
      <span
        v-for="b in bubbles"
        :key="b.key + b.value"
        class="inline-flex items-center gap-1 pl-2 pr-1 py-0.5 rounded-full bg-primary-500/10 text-primary-400 text-[11.5px] font-medium"
      >
        {{ b.label }}
        <button
          type="button"
          class="hover:text-text shrink-0"
          :aria-label="`Remove ${b.label}`"
          @click.stop="remove(b)"
        >
          <X class="w-3 h-3" />
        </button>
      </span>

      <input
        ref="inputEl"
        v-model="text"
        type="text"
        class="flex-1 min-w-[120px] bg-transparent outline-none text-text placeholder:text-text-muted py-1 text-[13px]"
        placeholder="Search"
        @keydown.enter.prevent="addName"
        @keydown.backspace="onBackspace"
      />

      <button
        type="button"
        class="shrink-0 flex items-center gap-1 px-2 py-1 rounded-lg text-text-muted hover:text-text transition-colors"
        aria-label="Filter options"
        title="More filters"
        @click.stop="showPopup = !showPopup"
      >
        <Filter class="w-3.5 h-3.5" />
        <ChevronDown class="w-3 h-3 transition-transform" :class="showPopup ? 'rotate-180' : ''" />
      </button>
    </div>

    <FilterTransactionsPopover
      v-if="showPopup"
      :filters="filters"
      :categories="categories"
      @apply="onApply"
      @close="showPopup = false"
    />
  </div>
</template>

