<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { ChevronLeft, ChevronRight } from '@lucide/vue';
import { formatCurrency } from '@/lib/utils/format';
import { isCredit } from '@/lib/utils/accountType';
import { useAccountsStore } from '@/lib/stores/accounts';

defineProps<{ accounts: any[] }>();

const store = useAccountsStore();
const scrollEl = ref<HTMLElement | null>(null);
const canScrollLeft = ref(false);
const canScrollRight = ref(false);

function accountLabel(a: any) {
  return a.accountNickname || a.bankName;
}

function updateScrollState() {
  const el = scrollEl.value;
  if (!el) return;
  canScrollLeft.value = el.scrollLeft > 2;
  canScrollRight.value = el.scrollLeft < el.scrollWidth - el.clientWidth - 2;
}

function scrollBy(px: number) {
  scrollEl.value?.scrollBy({ left: px, behavior: 'smooth' });
}

onMounted(() => {
  updateScrollState();
  scrollEl.value?.addEventListener('scroll', updateScrollState, { passive: true });
  window.addEventListener('resize', updateScrollState);
});
onUnmounted(() => {
  scrollEl.value?.removeEventListener('scroll', updateScrollState);
  window.removeEventListener('resize', updateScrollState);
});
</script>

<template>
  <div class="relative group rounded-2xl border border-border bg-base-200 min-h-[60px] flex items-center px-3 overflow-hidden">
    <div ref="scrollEl" class="flex flex-nowrap items-center gap-1.5 overflow-x-auto no-scrollbar">
      <button
        @click="store.select(null)"
        class="shrink-0 px-3 py-1.5 rounded-xl text-[13.5px] font-medium transition-all flex items-center gap-2"
        :class="store.state.selectedAccountId === null ? 'bg-primary-600 text-white' : 'bg-surface text-text-muted border border-border hover:border-primary-500/40'"
      >
        <span class="w-2 h-2 rounded-full" :class="store.state.selectedAccountId === null ? 'bg-white' : 'bg-track'"></span>
        All Accounts
      </button>
      <button
        v-for="acc in accounts"
        :key="acc.id"
        @click="store.select(acc.id)"
        class="shrink-0 px-3 py-1.5 rounded-xl text-[13.5px] font-medium transition-all flex items-center gap-2"
        :class="store.state.selectedAccountId === acc.id ? 'bg-primary-600 text-white' : 'bg-surface text-text-muted border border-border hover:border-primary-500/40'"
      >
        <span class="w-2 h-2 rounded-full" :class="store.state.selectedAccountId === acc.id ? 'bg-white' : isCredit(acc.accountType) ? 'bg-expense' : 'bg-income'"></span>
        {{ accountLabel(acc) }}
        <span class="font-semibold" :class="store.state.selectedAccountId === acc.id ? 'text-white' : isCredit(acc.accountType) ? 'text-expense' : 'text-income'">
          {{ formatCurrency(acc.balance ?? 0) }}
        </span>
      </button>
    </div>

    <button
      v-if="canScrollLeft"
      type="button"
      aria-label="Scroll left"
      class="absolute left-1 top-1/2 -translate-y-1/2 z-10 w-7 h-7 flex items-center justify-center rounded-full bg-base-200 border border-border text-text-muted shadow-popover opacity-0 group-hover:opacity-100 transition-opacity"
      @click="scrollBy(-55)"
    >
      <ChevronLeft class="w-4 h-4" stroke-width="2" />
    </button>
    <button
      v-if="canScrollRight"
      type="button"
      aria-label="Scroll right"
      class="absolute right-1 top-1/2 -translate-y-1/2 z-10 w-7 h-7 flex items-center justify-center rounded-full bg-base-200 border border-border text-text-muted shadow-popover opacity-0 group-hover:opacity-100 transition-opacity"
      @click="scrollBy(55)"
    >
      <ChevronRight class="w-4 h-4" stroke-width="2" />
    </button>

    <p v-if="accounts.length === 0" class="text-center py-4 text-subtle text-[13px]">No accounts yet. Add one to start tracking.</p>
  </div>
</template>
