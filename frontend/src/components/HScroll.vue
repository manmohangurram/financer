<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { ChevronLeft, ChevronRight } from '@lucide/vue';

// Reusable horizontal scroller: content scrolls with a hidden scrollbar; arrow
// buttons are invisible until you hover the row, then they appear and scroll
// `scrollAmount` px per click. Arrows hide automatically at the scroll ends.
// Extra attributes (class, style) fall through to the root box.
withDefaults(
  defineProps<{
    gap?: number; // spacing between items, px
    scrollAmount?: number; // px per arrow click
    align?: 'center' | 'start';
  }>(),
  { gap: 6, scrollAmount: 150, align: 'center' }
);

const scrollEl = ref<HTMLElement | null>(null);
const canScrollLeft = ref(false);
const canScrollRight = ref(false);

function updateState() {
  const el = scrollEl.value;
  if (!el) return;
  canScrollLeft.value = el.scrollLeft > 2;
  canScrollRight.value = el.scrollLeft < el.scrollWidth - el.clientWidth - 2;
}

function scrollBy(px: number) {
  scrollEl.value?.scrollBy({ left: px, behavior: 'smooth' });
}

onMounted(() => {
  updateState();
  scrollEl.value?.addEventListener('scroll', updateState, { passive: true });
  window.addEventListener('resize', updateState);
});
onUnmounted(() => {
  scrollEl.value?.removeEventListener('scroll', updateState);
  window.removeEventListener('resize', updateState);
});
</script>

<template>
  <div class="relative group">
    <div ref="scrollEl" class="overflow-x-auto no-scrollbar">
      <div
        class="flex w-max"
        :style="{ gap: `${gap}px` }"
        :class="[
          align === 'start' ? 'items-start' : 'items-center',
          !canScrollLeft && !canScrollRight ? 'mx-auto' : ''
        ]"
      >
        <slot />
      </div>
    </div>

    <button
      v-if="canScrollLeft"
      type="button"
      aria-label="Scroll left"
      class="absolute left-1 top-1/2 -translate-y-1/2 z-10 w-7 h-7 flex items-center justify-center rounded-full bg-base-200 border border-border text-text-muted shadow-popover opacity-0 group-hover:opacity-100 focus-visible:opacity-100 transition-opacity"
      @click="scrollBy(-scrollAmount)"
    >
      <ChevronLeft class="w-4 h-4" stroke-width="2" />
    </button>
    <button
      v-if="canScrollRight"
      type="button"
      aria-label="Scroll right"
      class="absolute right-1 top-1/2 -translate-y-1/2 z-10 w-7 h-7 flex items-center justify-center rounded-full bg-base-200 border border-border text-text-muted shadow-popover opacity-0 group-hover:opacity-100 focus-visible:opacity-100 transition-opacity"
      @click="scrollBy(scrollAmount)"
    >
      <ChevronRight class="w-4 h-4" stroke-width="2" />
    </button>
  </div>
</template>
