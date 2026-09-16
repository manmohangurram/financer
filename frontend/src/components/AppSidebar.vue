<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '@/lib/stores/auth';
import { useAccountsStore } from '@/lib/stores/accounts';
import { accounts as accountsApi } from '@/lib/api/client';
import Popover from '@/components/Popover.vue';
import pkg from '../../package.json';
import { NAV_SECTIONS, resolveActiveHref, sliceAccounts } from '@/lib/utils/sidebar';
import { isCredit } from '@/lib/utils/accountType';
import { formatCurrency } from '@/lib/utils/format';
import { Layers, Settings2, LogOut, ChevronUp, PanelLeftClose, PanelLeftOpen } from '@lucide/vue';

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: 'navigate'): void }>();

const ACCOUNT_ROWS = 5;

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const accountStore = useAccountsStore();
const APP_VERSION = pkg.version;

const userMenuOpen = ref(false);
const accountList = ref<{ id: string; nickname?: string; bankName?: string; type: string; balance?: number }[]>([]);

// Auto-collapse to the icon rail when there isn't room for the full sidebar
// (below lg). The persisted preference applies when wide.
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

function toggleCollapsed() {
  collapsed.value = !collapsed.value;
  try {
    localStorage.setItem('financer-sidebar-collapsed', collapsed.value ? '1' : '0');
  } catch {
    /* non-persistent is fine */
  }
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
    class="fixed md:static z-[58] h-full bg-base-200 border-r border-border flex flex-col flex-shrink-0 transition-[width,transform] duration-300"
    :inert="isSmall && !open"
    :class="[open ? 'translate-x-0' : '-translate-x-full md:translate-x-0', collapsed ? 'w-[76px] min-w-[76px]' : 'w-[300px] min-w-[300px]']"
  >
    <div :class="collapsed ? 'py-5 flex justify-center' : 'p-7 pb-5'">
      <div :class="collapsed ? 'flex flex-col items-center gap-2' : 'flex items-center gap-3.5'">
        <div class="w-[46px] h-[46px] rounded-[13px] bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center text-[22px]">
          <Layers class="w-5 h-5 text-white" stroke-width="2" />
        </div>
        <div v-if="!collapsed">
          <div class="text-xl font-bold text-base-content tracking-tight">Financer</div>
          <div class="text-[13px] text-subtle font-medium">Personal Finance <span class="text-faint">v{{ APP_VERSION }}</span></div>
        </div>
      </div>
    </div>

    <nav
      class="flex-1 overflow-y-auto flex flex-col"
      :class="collapsed ? 'px-3 items-center gap-1' : 'px-4'"
    >
      <template v-for="(section, i) in NAV_SECTIONS" :key="section.label ?? `section-${i}`">
        <template v-if="collapsed">
          <div v-if="section.label || i > 0" class="w-8 h-px bg-border my-2"></div>
        </template>
        <div v-else class="text-[12px] font-semibold text-faint tracking-[0.09em] uppercase px-4 pb-2" :class="i > 0 ? 'pt-5' : 'pt-1'">
          {{ section.label }}
        </div>

        <router-link
          v-for="item in section.items"
          :key="item.to"
          :to="item.to"
          :title="item.label"
          class="w-full flex items-center gap-3.5 py-3 rounded-[11px] text-base font-medium transition-colors"
          :class="[
            activeHref === item.to ? 'bg-primary-500/[0.13] text-primary-700 dark:text-primary-200' : 'text-text-muted hover:bg-white/5 hover:text-base-content',
            collapsed ? 'justify-center px-0' : 'px-4'
          ]"
          @click="closeDrawer"
        >
          <component :is="item.icon" class="w-[22px] h-[22px]" stroke-width="1.5" />
          <span v-if="!collapsed">{{ item.label }}</span>
        </router-link>
      </template>

      <template v-if="accountList.length">
        <div v-if="collapsed" class="w-8 h-px bg-border my-2"></div>
        <div v-else class="text-[12px] font-semibold text-faint tracking-[0.09em] uppercase px-4 pt-5 pb-2">Accounts</div>

        <template v-if="collapsed">
          <button
            v-for="acc in accountSlice.shown"
            :key="acc.id"
            type="button"
            :title="acc.nickname || acc.bankName"
            :aria-label="acc.nickname || acc.bankName"
            class="w-2.5 h-2.5 rounded-full my-1 transition-colors"
            :class="[isCredit(acc.type) ? 'bg-expense' : 'bg-income', accountStore.state.selectedAccountId === acc.id ? 'ring-2 ring-primary-500/60' : '']"
            @click="goToAccount(acc.id)"
          ></button>
          <router-link v-if="accountSlice.more" to="/accounts" class="text-[11px] font-semibold text-faint hover:text-base-content mt-1" @click="closeDrawer">
            +{{ accountSlice.more }}
          </router-link>
        </template>

        <template v-else>
          <button
            v-for="acc in accountSlice.shown"
            :key="acc.id"
            type="button"
            class="w-full flex items-center gap-3 px-4 py-2.5 rounded-[11px] text-left transition-colors"
            :class="accountStore.state.selectedAccountId === acc.id ? 'bg-primary-500/[0.13]' : 'hover:bg-white/5'"
            @click="goToAccount(acc.id)"
          >
            <span class="w-2 h-2 rounded-full shrink-0" :class="isCredit(acc.type) ? 'bg-expense' : 'bg-income'"></span>
            <span class="flex-1 min-w-0 text-[13.5px] font-medium text-text-muted truncate">{{ acc.nickname || acc.bankName }}</span>
            <span class="text-[12.5px] font-semibold shrink-0" :class="isCredit(acc.type) ? 'text-expense' : 'text-income'">
              {{ formatCurrency(acc.balance ?? 0) }}
            </span>
          </button>
          <router-link v-if="accountSlice.more" to="/accounts" class="px-4 py-2 text-[13px] font-semibold text-faint hover:text-base-content transition-colors" @click="closeDrawer">
            +{{ accountSlice.more }} more →
          </router-link>
        </template>
      </template>
    </nav>

    <div :class="collapsed ? 'px-2 pb-4' : 'px-4 pb-4'">
      <button
        type="button"
        class="w-full flex items-center gap-3.5 py-3.5 rounded-[11px] text-base font-medium text-text-muted hover:text-base-content hover:bg-white/5 transition-colors"
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
</template>
