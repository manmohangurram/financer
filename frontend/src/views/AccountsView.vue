<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import AppButton from '@/components/ui/AppButton.vue';
import AppPageHeader from '@/components/ui/AppPageHeader.vue';
import AppMoneyBand from '@/components/ui/AppMoneyBand.vue';
import AccountFilterBar from '@/components/accounts/AccountFilterBar.vue';
import AccountFormModal from '@/components/accounts/AccountFormModal.vue';
import TransactionsPanel from '@/components/workspace/TransactionsPanel.vue';
import { accounts } from '@/lib/api/client';
import { useAccountsStore } from '@/lib/stores/accounts';
import { formatAmount } from '@/lib/utils/format';
import { isCredit, accountTypeLabel } from '@/lib/utils/accountType';
import { Settings2, Landmark, Plus, Upload } from '@lucide/vue';

const router = useRouter();
const store = useAccountsStore();
const panel = ref<InstanceType<typeof TransactionsPanel> | null>(null);

const accountList = ref<any[]>([]);
const showAccountModal = ref(false);
const loadError = ref('');

// The money band follows the account chips. One figure leads, two facts explain
// it, one bar says how big it is against the book. The label and the facts move
// with the scope: a single account is a balance, the whole book is a net worth.
const selected = computed(() => accountList.value.find((a: any) => a.id === store.state.selectedAccountId) ?? null);
const creditSelected = computed(() => !!selected.value && isCredit(selected.value.type));

const totalCash = computed(() =>
  accountList.value.filter((a: any) => !isCredit(a.type)).reduce((s: number, a: any) => s + (a.balance ?? 0), 0)
);
const totalOwed = computed(() =>
  accountList.value.filter((a: any) => isCredit(a.type)).reduce((s: number, a: any) => s + Math.abs(a.balance ?? 0), 0)
);
const netWorth = computed(() => totalCash.value - totalOwed.value);

const TYPE_TAIL = (account: any) => (account.endingNumbers ? ` · ${account.endingNumbers}` : '');

const pct = (part: number, whole: number) => (whole > 0 ? Math.round((part / whole) * 100) : 0);

const headlineLabel = computed(() => (creditSelected.value ? 'Outstanding' : selected.value ? 'Balance' : 'Net worth'));
const headlineValue = computed(() => {
  if (!selected.value) return netWorth.value;
  const balance = selected.value.balance ?? 0;
  return creditSelected.value ? Math.abs(balance) : balance;
});

/** Two facts: what the scope is made of, in the words that fit it. */
const facts = computed<{ label: string; value: string; mono?: boolean; tone?: 'ink' | 'debit' }[]>(() => {
  if (!selected.value) {
    return [
      { label: 'Cash', value: money(totalCash.value) },
      { label: 'Owed', value: money(totalOwed.value), tone: 'debit' }
    ];
  }
  const type = accountTypeLabel(selected.value.type);
  const tail = TYPE_TAIL(selected.value);
  const balance = Math.abs(selected.value.balance ?? 0);
  return creditSelected.value
    ? [
        { label: 'Type', value: `${type}${tail}`, mono: false },
        { label: 'Share of debt', value: `${pct(balance, totalOwed.value)}%` }
      ]
    : [
        { label: 'Type', value: `${type}${tail}`, mono: false },
        { label: 'Holds', value: `${pct(balance, totalCash.value)}% of cash` }
      ];
});

/** The bar is always "how big is this against the book", never a zero-length stub. */
const bar = computed(() => {
  if (!selected.value) {
    const gross = totalCash.value + totalOwed.value;
    const cashPct = pct(totalCash.value, gross);
    return {
      main: cashPct,
      color: 'var(--accent)',
      rest: Math.max(0, 100 - cashPct),
      restColor: 'var(--debit)',
      note: `${cashPct}% cash · ${100 - cashPct}% owed · ${accountList.value.length} accounts`
    };
  }
  const balance = Math.abs(selected.value.balance ?? 0);
  if (creditSelected.value) {
    const share = pct(balance, totalOwed.value);
    return {
      main: share,
      color: 'var(--debit)',
      rest: Math.max(0, 100 - share),
      restColor: 'var(--rule)',
      note:
        share >= 100
          ? `all ${money(balance)} owed sits on this card · ${pct(balance, netWorth.value)}% of net worth`
          : `${share}% of the ${money(totalOwed.value)} owed · ${pct(balance, netWorth.value)}% of net worth`
    };
  }
  const share = pct(balance, netWorth.value);
  return {
    main: share,
    color: 'var(--accent)',
    rest: Math.max(0, 100 - share),
    restColor: 'var(--rule)',
    note: `${share}% of ${money(netWorth.value)} net worth · ${accountList.value.length} accounts in the book`
  };
});

const money = (value: number) => formatAmount(Math.round(value));

async function loadAll() {
  loadError.value = '';
  try {
    const accResp = await accounts().listAccounts({});
    accountList.value = accResp.accounts || [];
  } catch (e: any) {
    loadError.value = e?.message || 'Failed to load accounts';
  }
}

onMounted(loadAll);
</script>

<template>
  <div class="w-full flex h-full min-h-0 flex-col gap-4">
    <AppPageHeader eyebrow="Ledger" title="Accounts" subtitle="Track spending and transactions across your accounts">
      <template v-if="accountList.length > 0">
        <AppButton variant="outline" @click="router.push('/accounts/manage')">
          <Settings2 class="h-4 w-4" />
          Manage
        </AppButton>
        <AppButton variant="outline" @click="panel?.openImport()">
          <Upload class="h-4 w-4" />
          Import
        </AppButton>
        <AppButton @click="panel?.openAdd()">
          <Plus class="h-4 w-4" />
          Add
        </AppButton>
      </template>
    </AppPageHeader>

    <div v-if="loadError" class="rounded-card border border-debit/40 bg-debit/10 px-4 py-3 text-[13px] text-debit">
      {{ loadError }}
    </div>

    <div v-if="accountList.length === 0" class="flex items-center justify-center min-h-[calc(100vh-180px)]">
      <div class="rounded-card border border-rule bg-page px-10 py-12 text-center max-w-md w-full">
        <div class="w-14 h-14 mx-auto rounded-card bg-accent/10 text-accent flex items-center justify-center">
          <Landmark class="w-7 h-7" stroke-width="1.5" />
        </div>
        <h2 class="text-lg font-semibold text-ink mt-4">Add your first account</h2>
        <p class="text-[13px] text-ink-mute mt-1.5">
          Start tracking your money — add a bank account to begin recording transactions.
        </p>
        <AppButton variant="solid" class="mt-6" @click="showAccountModal = true">
          <Plus class="h-4 w-4" />
          Add Account
        </AppButton>
      </div>
    </div>

    <template v-else>
      <AppMoneyBand
        :label="headlineLabel"
        :value="money(headlineValue)"
        :value-tone="creditSelected ? 'debit' : 'ink'"
        :facts="facts"
        :bar="{ percent: bar.main, color: bar.color, restPercent: bar.rest, restColor: bar.restColor }"
        :note="bar.note"
      />

      <AccountFilterBar :accounts="accountList" />

      <TransactionsPanel ref="panel" @updated="loadAll" />
    </template>

    <AccountFormModal v-if="showAccountModal" :account="null" @close="showAccountModal = false" @saved="loadAll" />
  </div>
</template>
