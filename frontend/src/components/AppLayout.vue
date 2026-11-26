<script setup lang="ts">
import { ref, computed, nextTick, watch, onUnmounted } from 'vue';
import { LayoutDashboard, CreditCard, PieChart, MoreHorizontal } from '@lucide/vue';
import { useRoute } from 'vue-router';
import AppSidebar from '@/components/AppSidebar.vue';

// The drawer state lives here: the mobile bar, the burger and the overlay all
// drive it, the sidebar only renders it.
const sidebarOpen = ref(false);

// The destinations that earn a slot in the phone's bottom bar. Everything
// else is one tap away under More.
const BAR = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/accounts', label: 'Accounts', icon: CreditCard },
  { to: '/accounts/spending', label: 'Spending', icon: PieChart }
];

const route = useRoute();
const activeBarHref = computed(() => {
  const hits = BAR.map((b) => b.to).filter((href) => route.path === href || route.path.startsWith(`${href}/`));
  return hits.sort((a, b) => b.length - a.length)[0] ?? null;
});

const moreButton = ref<HTMLElement | null>(null);

function onDrawerKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    sidebarOpen.value = false;
    moreButton.value?.focus();
  }
}

// A drawer is a modal surface: focus moves into it, Escape closes it, and the
// page behind it is inert so Tab cannot wander off behind the overlay.
watch(sidebarOpen, async (open) => {
  if (open) {
    document.addEventListener('keydown', onDrawerKey);
    document.body.style.overflow = 'hidden';
    await nextTick();
    document.querySelector<HTMLElement>('aside a, aside button')?.focus();
  } else {
    document.removeEventListener('keydown', onDrawerKey);
    document.body.style.overflow = '';
  }
});
onUnmounted(() => {
  document.removeEventListener('keydown', onDrawerKey);
  document.body.style.overflow = '';
});
</script>

<template>
  <div class="flex h-dvh overflow-hidden bg-paper text-ink">
    <a
      href="#main-content"
      class="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[120] focus:rounded-control focus:border focus:border-accent focus:bg-page focus:px-3 focus:py-2 focus:text-[13px] focus:font-semibold focus:text-ink"
    >
      Skip to content
    </a>
    <div
      v-if="sidebarOpen"
      class="fixed inset-0 z-[55] bg-overlay md:hidden"
      @click="sidebarOpen = false"
      aria-hidden="true"
    ></div>

    <AppSidebar :open="sidebarOpen" @navigate="sidebarOpen = false" />

    <div class="flex min-w-0 flex-1 flex-col overflow-hidden" :inert="sidebarOpen">
      <main id="main-content" class="flex min-h-0 flex-1 flex-col overflow-hidden" tabindex="-1">
        <div class="flex w-full min-h-0 flex-1 flex-col overflow-y-auto px-4 pb-20 pt-5 md:px-7 md:py-6">
          <router-view />
        </div>
      </main>
    </div>

    <!-- phone: five destinations, everything else under More -->
    <nav
      class="fixed bottom-0 left-0 right-0 z-[56] flex border-t border-rule bg-rail md:hidden"
      aria-label="Primary"
      :inert="sidebarOpen"
    >
      <router-link
        v-for="item in BAR"
        :key="item.to"
        :to="item.to"
        class="relative flex min-h-11 flex-1 flex-col items-center justify-center gap-1 py-1.5 text-[9px] uppercase tracking-[0.1em]"
        :class="activeBarHref === item.to ? 'text-ink' : 'text-ink-mute'"
      >
        <span
          v-if="activeBarHref === item.to"
          class="absolute left-1/2 top-0 h-[2px] w-7 -translate-x-1/2 rounded-full bg-accent"
          aria-hidden="true"
        ></span>
        <component :is="item.icon" class="h-[17px] w-[17px]" stroke-width="1.6" aria-hidden="true" />
        <span class="truncate px-0.5">{{ item.label }}</span>
      </router-link>
      <button
        ref="moreButton"
        type="button"
        class="flex min-h-11 flex-1 flex-col items-center justify-center gap-1 py-1.5 text-[9px] uppercase tracking-[0.1em] text-ink-mute"
        :aria-expanded="sidebarOpen"
        @click="sidebarOpen = true"
      >
        <MoreHorizontal class="h-[17px] w-[17px]" stroke-width="1.6" aria-hidden="true" />
        More
      </button>
    </nav>
  </div>
</template>
