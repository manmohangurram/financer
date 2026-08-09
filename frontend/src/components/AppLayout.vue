<script setup lang="ts">
import { ref, computed, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '@/lib/stores/auth';
import Popover from '@/components/Popover.vue';
import pkg from '../../package.json';
import { Menu, Layers, LayoutDashboard, CreditCard, BarChart3, Settings2, LogOut, ChevronUp, PanelLeftClose, PanelLeftOpen } from '@lucide/vue';
const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const APP_VERSION = pkg.version;
const sidebarOpen = ref(false);
const userMenuOpen = ref(false);

// Auto-collapse to the icon rail when there isn't room for the full sidebar
// (below lg). The persisted preference applies when wide.
const mq = window.matchMedia('(max-width: 1023px)');
const collapsed = ref(mq.matches ? true : localStorage.getItem('financer-sidebar-collapsed') === '1');
const onMqChange = (e: MediaQueryListEvent) => {
  if (e.matches) collapsed.value = true;
};
mq.addEventListener('change', onMqChange);
onUnmounted(() => mq.removeEventListener('change', onMqChange));

const isActive = (href: string) =>
  route.path === href || (href !== '/' && route.path.startsWith(href));

const userInitials = computed(() =>
  (auth.state.user?.name || auth.state.user?.email || '?')
    .split(' ')
    .map((p: string) => p[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
);

function toggleCollapsed() {
  collapsed.value = !collapsed.value;
  try {
    localStorage.setItem('financer-sidebar-collapsed', collapsed.value ? '1' : '0');
  } catch {
    /* non-persistent is fine */
  }
}

function handleLogout() {
  auth.logout();
  router.replace('/login');
}
</script>

<template>
  <div class="flex h-screen overflow-hidden">
    <button
      class="btn btn-ghost btn-sm md:hidden fixed top-4 left-4 z-[60] text-base-content"
      aria-label="Toggle menu"
      @click="sidebarOpen = !sidebarOpen"
    >
      <Menu class="w-6 h-6" />
    </button>

    <div
      v-if="sidebarOpen"
      class="fixed inset-0 z-[55] bg-black/50 md:hidden"
      @click="sidebarOpen = false"
      aria-label="Close sidebar"
    ></div>

    <aside
      class="fixed md:static z-[58] h-full bg-base-200 border-r border-border flex flex-col flex-shrink-0 transition-[width,transform] duration-300"
      :class="[
        sidebarOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0',
        collapsed ? 'w-[76px] min-w-[76px]' : 'w-[300px] min-w-[300px]'
      ]"
    >
      <div :class="collapsed ? 'py-5 flex justify-center' : 'p-7 pb-5'">
        <div :class="collapsed ? 'flex flex-col items-center gap-2' : 'flex items-center gap-3.5'">
          <div class="w-[46px] h-[46px] rounded-[13px] bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center text-[22px]">
            <Layers class="w-5 h-5 text-white" stroke-width="2" />
          </div>
          <div v-if="!collapsed">
            <div class="text-[21px] font-bold text-base-content tracking-tight">Financer</div>
            <div class="text-[13px] text-subtle font-medium">Personal Finance <span class="text-faint">v{{ APP_VERSION }}</span></div>
          </div>
        </div>
      </div>

      <div v-if="!collapsed" class="text-[12px] font-semibold text-faint tracking-[0.09em] uppercase px-8 pb-3 pt-1">Menu</div>

      <nav class="flex-1 flex flex-col gap-[3px]" :class="collapsed ? 'px-3 items-center' : 'px-4'">
        <router-link
          to="/dashboard"
          title="Dashboard"
          class="w-full flex items-center gap-3.5 py-3.5 rounded-[11px] text-[16.5px] font-medium transition-colors"
          :class="[
            isActive('/dashboard') ? 'bg-primary-500/[0.13] text-primary-700 dark:text-primary-200' : 'text-text-muted hover:bg-white/5 hover:text-base-content',
            collapsed ? 'justify-center px-0' : 'px-4'
          ]"
        >
          <LayoutDashboard class="w-[22px] h-[22px]" stroke-width="1.5" />
          <span v-if="!collapsed">Dashboard</span>
        </router-link>

        <router-link
          to="/accounts"
          title="Accounts"
          class="w-full flex items-center gap-3.5 py-3.5 rounded-[11px] text-[16.5px] font-medium transition-colors"
          :class="[
            isActive('/accounts') ? 'bg-primary-500/[0.13] text-primary-700 dark:text-primary-200' : 'text-text-muted hover:bg-white/5 hover:text-base-content',
            collapsed ? 'justify-center px-0' : 'px-4'
          ]"
        >
          <CreditCard class="w-[22px] h-[22px]" stroke-width="1.5" />
          <span v-if="!collapsed">Accounts</span>
        </router-link>

        <router-link
          to="/investments"
          title="Investments"
          class="w-full flex items-center gap-3.5 py-3.5 rounded-[11px] text-[16.5px] font-medium transition-colors"
          :class="[
            isActive('/investments') ? 'bg-primary-500/[0.13] text-primary-700 dark:text-primary-200' : 'text-text-muted hover:bg-white/5 hover:text-base-content',
            collapsed ? 'justify-center px-0' : 'px-4'
          ]"
        >
          <BarChart3 class="w-[22px] h-[22px]" stroke-width="1.5" />
          <span v-if="!collapsed">Investments</span>
        </router-link>
      </nav>

      <div :class="collapsed ? 'px-2 pb-4' : 'px-4 pb-4'">
        <button
          type="button"
          class="w-full flex items-center gap-3.5 py-3.5 rounded-[11px] text-[16.5px] font-medium text-text-muted hover:text-base-content hover:bg-white/5 transition-colors"
          :class="collapsed ? 'justify-center' : 'px-4'"
          :aria-label="collapsed ? 'Expand sidebar' : 'Collapse sidebar'"
          @click="toggleCollapsed"
        >
          <PanelLeftClose v-if="!collapsed" class="w-[22px] h-[22px]" stroke-width="1.5" />
          <PanelLeftOpen v-else class="w-[22px] h-[22px]" stroke-width="1.5" />
          <span v-if="!collapsed">Collapse</span>
        </button>

        <div class="relative">
          <button
            class="w-full flex items-center gap-3.5 p-2.5 rounded-xl bg-white/[0.02] hover:bg-white/[0.05] transition-colors text-left"
            :class="collapsed ? 'justify-center' : ''"
            :aria-expanded="userMenuOpen"
            @click="userMenuOpen = !userMenuOpen"
          >
            <div class="w-[42px] h-[42px] rounded-full bg-accent flex items-center justify-center text-[15px] font-bold text-accent-content shrink-0">
              {{ userInitials }}
            </div>
            <template v-if="!collapsed">
              <div class="flex-1 min-w-0">
                <div class="text-[15px] font-semibold text-text truncate">{{ auth.state.user?.name || 'Account' }}</div>
                <div class="text-[12.5px] text-subtle truncate">{{ auth.state.user?.email || '' }}</div>
              </div>
              <ChevronUp class="w-4 h-4 text-subtle shrink-0 transition-transform" :class="userMenuOpen ? 'rotate-180' : ''" />
            </template>
          </button>

          <Popover v-if="userMenuOpen" right @close="userMenuOpen = false" panel-class="w-48">
            <div class="py-1">
              <button
                @click="router.push('/settings'); userMenuOpen = false"
                class="w-full flex items-center gap-3 px-4 py-2.5 text-[13.5px] font-medium text-left text-text-muted hover:text-primary-200 hover:bg-white/5 transition-colors"
              >
                <Settings2 class="w-[18px] h-[18px]" stroke-width="1.5" />
                Settings
              </button>
              <button
                @click="handleLogout"
                class="w-full flex items-center gap-3 px-4 py-2.5 text-[13.5px] font-medium text-left text-text-muted hover:text-expense hover:bg-expense/10 transition-colors"
              >
                <LogOut class="w-[18px] h-[18px]" stroke-width="1.5" />
                Logout
              </button>
            </div>
          </Popover>
        </div>
      </div>

    </aside>

    <div class="flex-1 flex flex-col overflow-hidden" style="background: radial-gradient(ellipse at top, var(--color-bg-elevated) 0%, var(--color-bg) 55%);">
      <main class="flex-1 overflow-auto p-7 w-full">
        <router-view />
      </main>
    </div>
  </div>
</template>
