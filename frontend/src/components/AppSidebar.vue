<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '@/lib/stores/auth';
import { useAccountsStore } from '@/lib/stores/accounts';
import { accounts as accountsApi } from '@/lib/api/client';
import Popover from '@/components/Popover.vue';
import { NAV_SECTIONS, resolveActiveHref, sliceAccounts } from '@/lib/utils/sidebar';
import { formatCurrency } from '@/lib/utils/format';
import { Settings2, LogOut, ChevronUp, PanelLeftClose, PanelLeftOpen, Sun, Moon } from '@lucide/vue';
import { current as themeMode, resolved as themeResolved, applyTheme } from '@/lib/theme';

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: 'navigate'): void }>();

const ACCOUNT_ROWS = 5;

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const accountStore = useAccountsStore();

const userMenuOpen = ref(false);
const accountList = ref<{ id: string; nickname?: string; bankName?: string; balance?: number }[]>([]);

// Below lg there is no room for the full sidebar, so it becomes a drawer (the
// mobile bar in AppLayout drives it) and starts collapsed.
const mq = window.matchMedia('(max-width: 1023px)');
const collapsed = ref(mq.matches ? true : localStorage.getItem('financer-sidebar-collapsed') === '1');
const isSmall = ref(mq.matches);
const onMqChange = (e: MediaQueryListEvent) => {
  isSmall.value = e.matches;
  if (e.matches) collapsed.value = true;
};
mq.addEventListener('change', onMqChange);
onUnmounted(() => mq.removeEventListener('change', onMqChange));

const activeHref = computed(() => resolveActiveHref(route.path));
const accountSlice = computed(() => sliceAccounts(accountList.value, ACCOUNT_ROWS));

const userInitials = computed(() =>
  (auth.state.user?.name || auth.state.user?.email || '?')
    .split(' ')
    .map((p: string) => p[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
);

const isDark = computed(() => themeResolved.value === 'dark');
const isNegative = (balance?: number) => (balance ?? 0) < 0;

function toggleCollapsed() {
  collapsed.value = !collapsed.value;
  try {
    localStorage.setItem('financer-sidebar-collapsed', collapsed.value ? '1' : '0');
  } catch {
    /* non-persistent is fine */
  }
}

function toggleTheme() {
  applyTheme(isDark.value ? 'light' : 'dark');
}

function closeDrawer() {
  if (props.open) emit('navigate');
}

function goToAccount(id: string) {
  accountStore.select(id);
  router.push('/accounts');
  closeDrawer();
}

function handleLogout() {
  auth.logout();
  router.replace('/login');
}

onMounted(async () => {
  try {
    const resp = await accountsApi().listAccounts({});
    accountList.value = resp.accounts || [];
  } catch {
    // The nav is still usable without the balance list.
  }
});
</script>

<template>
  <aside
    data-testid="sidebar"
    class="fixed md:static z-[58] h-full bg-paper border-r border-rule flex flex-col flex-shrink-0 transition-[width,transform] duration-300"
    :inert="isSmall && !open"
    :class="[open ? 'translate-x-0' : '-translate-x-full md:translate-x-0', collapsed ? 'w-[68px] min-w-[68px]' : 'w-[244px] min-w-[244px]']"
  >
    <!-- the mark: a folio page whose margin carries the clerk's tick -->
    <router-link
      to="/dashboard"
      class="flex items-center gap-2.5 border-b border-rule-soft text-ink"
      :class="collapsed ? 'justify-center px-2 py-4' : 'px-5 py-4'"
      @click="closeDrawer"
    >
      <svg viewBox="0 0 64 64" class="w-[26px] h-[26px] shrink-0" aria-hidden="true">
        <rect x="8" y="5" width="48" height="54" rx="5" fill="none" stroke="currentColor" stroke-width="4.5" />
        <rect x="12" y="9" width="13" height="46" rx="2" fill="currentColor" />
        <polyline points="14,31 17.5,35 22,25.5" fill="none" stroke="var(--page)" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
        <line x1="30" y1="20" x2="50" y2="20" stroke="currentColor" stroke-width="3" />
        <line x1="30" y1="32" x2="44" y2="32" stroke="currentColor" stroke-width="3" />
        <line x1="30" y1="44" x2="47" y2="44" stroke="currentColor" stroke-width="3" />
      </svg>
      <span v-if="!collapsed" class="font-display text-[21px] leading-none tracking-[-0.02em]">financer</span>
    </router-link>

    <nav class="flex-1 overflow-y-auto py-3" :class="collapsed ? 'px-2' : 'px-3'">
      <template v-for="(section, i) in NAV_SECTIONS" :key="section.label ?? `section-${i}`">
        <div v-if="collapsed && i > 0" class="mx-auto my-3 h-px w-6 bg-rule"></div>
        <div
          v-else-if="section.label"
          class="px-2 pb-1.5 text-label font-medium uppercase tracking-[0.06em] text-ink-faint"
          :class="i > 0 ? 'pt-5' : 'pt-1'"
        >
          {{ section.label }}
        </div>

        <router-link
          v-for="item in section.items"
          :key="item.to"
          :to="item.to"
          :title="item.label"
          class="group relative flex items-center gap-3 rounded-control py-2 font-medium transition-colors"
          :class="[
            activeHref === item.to ? 'bg-page-sunk text-ink' : 'text-ink-soft hover:bg-page-sunk/70 hover:text-ink',
            collapsed ? 'justify-center px-0' : 'px-2.5'
          ]"
          @click="closeDrawer"
        >
          <span
            v-if="activeHref === item.to"
            class="absolute left-0 top-1.5 bottom-1.5 w-[3px] rounded-full bg-accent"
            aria-hidden="true"
          ></span>
          <component :is="item.icon" class="w-[17px] h-[17px] shrink-0" stroke-width="1.6" />
          <span v-if="!collapsed" class="truncate">{{ item.label }}</span>
        </router-link>
      </template>

      <template v-if="accountList.length">
        <div v-if="collapsed" class="mx-auto my-3 h-px w-6 bg-rule"></div>
        <div v-else class="px-2 pb-1.5 pt-5 text-label font-medium uppercase tracking-[0.06em] text-ink-faint">Accounts</div>

        <template v-if="collapsed">
          <button
            v-for="acc in accountSlice.shown"
            :key="acc.id"
            type="button"
            :title="acc.nickname || acc.bankName"
            :aria-label="acc.nickname || acc.bankName"
            class="mx-auto my-1 block h-2.5 w-2.5 rounded-full transition-colors"
            :class="[isNegative(acc.balance) ? 'bg-debit' : 'bg-credit', accountStore.state.selectedAccountId === acc.id ? 'ring-2 ring-accent/60' : '']"
            @click="goToAccount(acc.id)"
          ></button>
        </template>

        <template v-else>
          <button
            v-for="acc in accountSlice.shown"
            :key="acc.id"
            type="button"
            class="flex w-full items-center gap-2.5 rounded-control px-2.5 py-1.5 text-left transition-colors"
            :class="accountStore.state.selectedAccountId === acc.id ? 'bg-page-sunk' : 'hover:bg-page-sunk/70'"
            @click="goToAccount(acc.id)"
          >
            <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="isNegative(acc.balance) ? 'bg-debit' : 'bg-credit'"></span>
            <span class="min-w-0 flex-1 truncate text-[13px] text-ink-soft">{{ acc.nickname || acc.bankName }}</span>
            <span class="tabular shrink-0 text-[12px] font-medium" :class="isNegative(acc.balance) ? 'text-debit' : 'text-credit'">
              {{ formatCurrency(acc.balance ?? 0) }}
            </span>
          </button>
          <router-link
            v-if="accountSlice.more"
            to="/accounts"
            class="block px-2.5 py-1 text-[12px] text-ink-mute transition-colors hover:text-ink"
            @click="closeDrawer"
          >
            +{{ accountSlice.more }} more
          </router-link>
        </template>
      </template>
    </nav>

    <div class="border-t border-rule-soft p-2" :class="collapsed ? 'space-y-1' : 'space-y-1'">
      <div class="flex items-center gap-1" :class="collapsed ? 'flex-col' : ''">
        <button
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-control text-ink-mute transition-colors hover:bg-page-sunk hover:text-ink"
          :aria-label="isDark ? 'Switch to paper theme' : 'Switch to night theme'"
          :title="isDark ? 'Paper theme' : 'Night theme'"
          @click="toggleTheme"
        >
          <Sun v-if="isDark" class="h-[15px] w-[15px]" stroke-width="1.6" />
          <Moon v-else class="h-[15px] w-[15px]" stroke-width="1.6" />
        </button>
        <button
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-control text-ink-mute transition-colors hover:bg-page-sunk hover:text-ink"
          :aria-label="collapsed ? 'Expand sidebar' : 'Collapse sidebar'"
          @click="toggleCollapsed"
        >
          <PanelLeftOpen v-if="collapsed" class="h-[15px] w-[15px]" stroke-width="1.6" />
          <PanelLeftClose v-else class="h-[15px] w-[15px]" stroke-width="1.6" />
        </button>
        <span v-if="!collapsed" class="ml-auto pr-1 font-mono text-[10px] text-ink-faint">{{ themeMode }}</span>
      </div>

      <div class="relative">
        <button
          class="flex w-full items-center gap-2.5 rounded-control p-1.5 text-left transition-colors hover:bg-page-sunk"
          :class="collapsed ? 'justify-center' : ''"
          :aria-expanded="userMenuOpen"
          @click="userMenuOpen = !userMenuOpen"
        >
          <span
            class="flex h-7 w-7 shrink-0 items-center justify-center rounded-[4px] border border-rule bg-page-sunk text-[11px] font-semibold text-ink-soft"
          >
            {{ userInitials }}
          </span>
          <template v-if="!collapsed">
            <span class="min-w-0 flex-1">
              <span class="block truncate text-[13px] font-medium text-ink">{{ auth.state.user?.name || 'Account' }}</span>
              <span class="block truncate text-[11px] text-ink-mute">{{ auth.state.user?.email || '' }}</span>
            </span>
            <ChevronUp class="h-3.5 w-3.5 shrink-0 text-ink-faint transition-transform" :class="userMenuOpen ? 'rotate-180' : ''" />
          </template>
        </button>

        <Popover v-if="userMenuOpen" right @close="userMenuOpen = false" panel-class="w-44">
          <div class="py-1">
            <button
              class="flex w-full items-center gap-2.5 px-3 py-2 text-left text-[13px] text-ink-soft transition-colors hover:bg-page-sunk hover:text-ink"
              @click="router.push('/settings'); userMenuOpen = false"
            >
              <Settings2 class="h-[15px] w-[15px]" stroke-width="1.6" />
              Settings
            </button>
            <button
              class="flex w-full items-center gap-2.5 px-3 py-2 text-left text-[13px] text-ink-soft transition-colors hover:bg-page-sunk hover:text-debit"
              @click="handleLogout"
            >
              <LogOut class="h-[15px] w-[15px]" stroke-width="1.6" />
              Log out
            </button>
          </div>
        </Popover>
      </div>
    </div>
  </aside>
</template>
