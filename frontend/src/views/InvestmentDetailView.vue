<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import InvestmentDetail from '@/components/investments/InvestmentDetail.vue';
import InvestmentModal from '@/components/investments/InvestmentModal.vue';
import LotModal from '@/components/investments/LotModal.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { investments } from '@/lib/api/client';

const route = useRoute();
const router = useRouter();

const loading = ref(true);
const error = ref('');
const investment = ref<any>(null);
const lots = ref<any[]>([]);
const showInvestmentModal = ref(false);
const showLotModal = ref(false);
const lotSide = ref<'buy' | 'sell'>('buy');
const editingLot = ref<any | null>(null);
const confirmDelete = ref<{ kind: 'investment' | 'lot'; item: any } | null>(null);

async function loadInvestment() {
  loading.value = true;
  error.value = '';
  try {
    investment.value = await investments().getInvestment({ id: route.params.id });
  } catch (e: any) {
    error.value = e?.message || 'Failed to load investment';
  }
  loading.value = false;
}

async function loadLots(id: string) {
  try {
    const resp = await investments().listLots({ id });
    lots.value = resp.lots || [];
  } catch (e) {
    lots.value = [];
  }
}

async function load() {
  await loadInvestment();
  if (investment.value?.id) await loadLots(investment.value.id);
}

function openEdit() {
  showInvestmentModal.value = true;
}
function addLot(side: 'buy' | 'sell') {
  editingLot.value = null;
  lotSide.value = side;
  showLotModal.value = true;
}
function editLot(lot: any) {
  editingLot.value = lot;
  lotSide.value = lot.side === 1 ? 'buy' : 'sell';
  showLotModal.value = true;
}

async function handleInvestmentSubmit(payload: any) {
  try {
    await investments().updateInvestment(payload);
    showInvestmentModal.value = false;
    await loadInvestment();
  } catch (e: any) {
    error.value = e?.message || 'Failed to save investment';
  }
}

async function handleLotSubmit(payload: any) {
  try {
    await investments().addLot(payload);
    showLotModal.value = false;
    await load();
  } catch (e: any) {
    error.value = e?.message || 'Failed to add lot';
  }
}

async function handleLotUpdate(payload: any) {
  try {
    await investments().updateLot({ id: investment.value?.id, lotId: payload.lotId, quantity: payload.quantity, price: payload.price, occurredAt: payload.occurredAt });
    showLotModal.value = false;
    await load();
  } catch (e: any) {
    error.value = e?.message || 'Failed to update lot';
  }
}

function requestDelete(kind: 'investment' | 'lot', item: any) {
  confirmDelete.value = { kind, item };
}

async function confirmDeleteNow() {
  if (!confirmDelete.value) return;
  const { kind, item } = confirmDelete.value;
  try {
    if (kind === 'investment') {
      await investments().deleteInvestment({ id: item.id });
      router.push('/investments');
    } else {
      await investments().deleteLot({ id: investment.value?.id, lotId: item.id });
      showLotModal.value = false;
      editingLot.value = null;
      await load();
    }
  } catch (e: any) {
    error.value = e?.message || 'Failed to delete';
  }
  confirmDelete.value = null;
}

onMounted(load);
</script>

<template>
  <div class="w-full space-y-5">
    <div v-if="error" class="rounded-xl border border-expense/30 bg-expense/10 px-4 py-3 text-[13px] text-expense">{{ error }}</div>

    <div v-if="loading" class="text-center py-16 text-subtle">Loading…</div>
    <div v-else-if="!investment" class="text-center py-16">
      <p class="text-subtle text-[14px]">Investment not found.</p>
      <button class="btn btn-ghost btn-sm text-primary-400 mt-3" @click="router.push('/investments')">Back to investments</button>
    </div>
    <InvestmentDetail
      v-else
      :investment="investment"
      :lots="lots"
      @close="router.push('/investments')"
      @edit="openEdit"
      @add-buy="addLot('buy')"
      @add-sell="addLot('sell')"
      @edit-lot="editLot"
    />

    <InvestmentModal
      v-if="showInvestmentModal"
      :investment="investment"
      @close="showInvestmentModal = false"
      @submit="handleInvestmentSubmit"
      @delete="requestDelete('investment', investment)"
    />
    <LotModal
      v-if="showLotModal && investment"
      :investment="investment"
      :side="lotSide"
      :lot="editingLot"
      @close="showLotModal = false"
      @submit="handleLotSubmit"
      @update="handleLotUpdate"
      @delete="requestDelete('lot', editingLot)"
    />

    <ConfirmDialog
      v-if="confirmDelete"
      :title="confirmDelete.kind === 'investment' ? 'Delete investment' : 'Delete lot'"
      :message="
        confirmDelete.kind === 'investment'
          ? `Delete &quot;${confirmDelete.item.name}&quot; and all its lots? This can't be undone.`
          : `Delete this ${confirmDelete.item.side === 1 ? 'buy' : 'sell'} lot? This can't be undone.`
      "
      @confirm="confirmDeleteNow"
      @cancel="confirmDelete = null"
    />
  </div>
</template>
