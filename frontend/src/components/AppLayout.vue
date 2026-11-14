<script setup lang="ts">
import { ref, computed } from 'vue';
import { LayoutDashboard, CreditCard, PieChart, Inbox, BarChart3, MoreHorizontal } from '@lucide/vue';
import { useRoute } from 'vue-router';
import AppSidebar from '@/components/AppSidebar.vue';

// The drawer state lives here: the mobile bar, the burger and the overlay all
// drive it, the sidebar only renders it.
const sidebarOpen = ref(false);

// The five destinations that earn a slot in the phone's bottom bar. Everything
// else is one tap away under More.
const BAR = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/accounts', label: 'Accounts', icon: CreditCard },
  { to: '/accounts/spending', label: 'Spending', icon: PieChart },
  { to: '/inbox', label: 'Inbox', icon: Inbox },
  { to: '/investments', label: 'Investments', icon: BarChart3 }
];

const route = useRoute();
const activeBarHref = computed(() => {
  const hits = BAR.map((b) => b.to).filter((href) => route.path === href || route.path.startsWith(`${href}/`));
  return hits.sort((a, b) => b.length - a.length)[0] ?? null;
});
</script>

<template>
  <div class="flex h-dvh overflow-hidden bg-paper text-ink">
    <div
      v-if="sidebarOpen"
      class="fixed inset-0 z-[55] bg-overlay md:hidden"
      @click="sidebarOpen = false"
      aria-hidden="true"
    ></div>

    <AppSidebar :open="sidebarOpen" @navigate="sidebarOpen = false" />

    <div class="flex min-w-0 flex-1 flex-col overflow-hidden">
      <main class="flex-1 overflow-auto">
        <div class="mx-auto w-full max-w-[1180px] px-4 pb-24 pt-6 md:px-8 md:py-8">
          <router-view />
        </div>
      </main>
    </div>

    <!-- phone: five destinations, everything else under More -->
    <nav
      class="fixed bottom-0 left-0 right-0 z-[56] flex border-t border-rule bg-page md:hidden"
      aria-label="Primary"
    >
      <router-link
        v-for="item in BAR"
        :key="item.to"
        :to="item.to"
        class="relative flex flex-1 flex-col items-center justify-center gap-1 py-1.5 text-[9px] uppercase tracking-[0.05em]"
        :class="activeBarHref === item.to ? 'text-ink' : 'text-ink-mute'"
      >
        <span
          v-if="activeBarHref === item.to"
          class="absolute left-1/2 top-0 h-[2px] w-7 -translate-x-1/2 rounded-full bg-accent"
          aria-hidden="true"
        ></span>
        <component :is="item.icon" class="h-[17px] w-[17px]" stroke-width="1.6" />
        <span class="truncate px-0.5">{{ item.label }}</span>
      </router-link>
      <button
        type="button"
        class="flex flex-1 flex-col items-center justify-center gap-1 py-1.5 text-[9px] uppercase tracking-[0.05em] text-ink-mute"
        :aria-expanded="sidebarOpen"
        @click="sidebarOpen = true"
      >
        <MoreHorizontal class="h-[17px] w-[17px]" stroke-width="1.6" />
        More
      </button>
    </nav>
  </div>
</template>
