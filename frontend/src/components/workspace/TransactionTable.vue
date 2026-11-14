<script setup lang="ts">
import { computed, ref } from 'vue';
import { formatCurrency, formatDate } from '@/lib/utils/format';
import { categoryColorMap } from '@/lib/utils/categoryColor';
import { ArrowLeftRight, Pencil } from '@lucide/vue';

const props = defineProps<{
  transactions: any[];
  categories: any[];
  selectedIds?: string[];
  readonly?: boolean;
  scrollable?: boolean;
  emptyText?: string;
  transferTxns?: any[];
  accounts?: any[];
  minRows?: number;
}>();

const emit = defineEmits<{
  (e: 'toggle-select', txn: any): void;
  (e: 'toggle-select-all'): void;
  (e: 'edit', txn: any): void;
  (e: 'transfer', txn: any): void;
}>();

const revealed = ref<string | null>(null);

const colors = computed(() => categoryColorMap(props.categories.map((c: any) => c.name)));

const allSelected = computed(() => props.transactions.length > 0 && props.transactions.every((t) => props.selectedIds?.includes(t.id)));
const someSelected = computed(() => !allSelected.value && props.transactions.some((t) => props.selectedIds?.includes(t.id)));
const hasSelection = computed(() => (props.selectedIds?.length ?? 0) > 0);
const colCount = computed(() => (props.readonly ? 4 : 6));
const fillRows = computed(() => Array(Math.max(0, (props.minRows || 0) - props.transactions.length)));

function isCredit(t: any) {
  return t.type === 'CREDIT';
}

function txnCategory(txn: any) {
  return txn.categoryIds?.length ? props.categories.find((c: any) => txn.categoryIds.includes(c.id)) : null;
}

function openRow(txn: any) {
  if (props.readonly) return;
  emit('edit', txn);
}

function counterpart(txn: any) {
  if (!txn.linkedTransferId) return null;
  const other = (props.transferTxns || []).find(
    (t) => t.linkedTransferId === txn.linkedTransferId && t.id !== txn.id
  );
  if (!other) return null;
  const acc = (props.accounts || []).find((a) => a.id === other.accountId);
  return { name: other.name, account: acc ? acc.nickname || acc.bankName : '' };
}
</script>

<template>
  <div class="w-full" :class="scrollable ? 'max-h-56 overflow-auto' : 'overflow-x-auto'">
    <table class="w-full table-fixed">
      <thead>
        <tr class="border-b border-rule bg-page-sunk text-label font-medium uppercase tracking-[0.06em] text-ink-mute" :class="scrollable && 'sticky top-0 z-10'">
          <th v-if="!readonly" class="w-8 py-2.5 pl-4">
            <input
              type="checkbox"
              class="h-3.5 w-3.5 accent-[var(--accent)] transition-opacity"
              :class="hasSelection ? 'opacity-100' : 'opacity-0 group-hover:opacity-100 focus-visible:opacity-100'"
              :checked="allSelected"
              :indeterminate="someSelected"
              @change="emit('toggle-select-all')"
              aria-label="Select all"
            />
          </th>
          <th class="px-2 py-2.5 text-left w-[20%] lg:w-[10%]">Date</th>
          <th class="px-2 py-2.5 text-left w-[50%] lg:w-[40%]">Name</th>
          <th class="hidden px-2 py-2.5 text-left lg:table-cell lg:w-[30%]">Categories</th>
          <th class="text-right py-2.5 px-3" :class="readonly ? 'w-[30%] lg:w-[20%]' : 'w-[20%] lg:w-[15%]'">Amount</th>
          <th v-if="!readonly" class="w-10 px-2 py-2.5" aria-label="Row actions"></th>
        </tr>
      </thead>
      <tbody class="divide-y divide-rule-soft">
        <tr
          v-for="txn in transactions"
          :key="txn.id"
          class="group h-11 transition-colors"
          :class="[
            !readonly && 'cursor-pointer',
            selectedIds?.includes(txn.id) ? 'bg-accent-wash hover:bg-accent-wash' : 'hover:bg-page-sunk/60'
          ]"
          @click="openRow(txn)"
        >
          <td v-if="!readonly" class="py-3 pl-4 w-8" @click.stop>
            <input
              type="checkbox"
              class="h-3.5 w-3.5 accent-[var(--accent)] transition-opacity"
              :class="hasSelection ? 'opacity-100' : 'opacity-0 group-hover:opacity-100 focus-visible:opacity-100'"
              :checked="selectedIds?.includes(txn.id)"
              @change="emit('toggle-select', txn)"
              aria-label="Select row"
            />
          </td>
          <td class="tabular px-2 py-2 text-[12px] whitespace-nowrap text-ink-mute">{{ formatDate(txn.occurredAt || '') }}</td>
          <td class="py-3 px-2">
            <div class="flex items-center gap-3">
              <div class="min-w-0">
                <div class="flex items-center gap-1.5">
                  <span class="truncate text-[13px] text-ink">{{ txn.name }}</span>
                  <button
                    v-if="!readonly && txn.type === 'DEBIT' && !txn.linkedTransferId"
                    type="button"
                    class="ns-icon opacity-0 group-hover:opacity-100 focus-visible:opacity-100 text-ink-soft shrink-0"
                    aria-label="Transfer this transaction"
                    title="Transfer to another account"
                    @click.stop="emit('transfer', txn)"
                  >
                    <ArrowLeftRight class="w-3.5 h-3.5" />
                  </button>
                  <button
                    v-if="txn.linkedTransferId && counterpart(txn)"
                    type="button"
                    class="ns-icon text-ink-soft shrink-0"
                    aria-label="Show linked transfer"
                    @click="revealed = revealed === txn.id ? null : txn.id"
                  >
                    <ArrowLeftRight class="w-3.5 h-3.5" />
                  </button>
                </div>
                <div v-if="revealed === txn.id && counterpart(txn)" class="text-[11px] text-ink-soft mt-0.5 whitespace-nowrap">
                  ⇄ {{ counterpart(txn)?.name }}<span v-if="counterpart(txn)?.account"> · {{ counterpart(txn)?.account }}</span>
                </div>
              </div>
            </div>
          </td>
          <td class="py-3 px-2 hidden lg:table-cell">
            <span
              v-if="txnCategory(txn)"
              class="badge badge-outline gap-1 text-[11px]"
              :style="{ backgroundColor: colors[txnCategory(txn).name] + '18', color: colors[txnCategory(txn).name], borderColor: colors[txnCategory(txn).name] + '40' }"
            >{{ txnCategory(txn).name }}</span>
          </td>
          <td class="py-3 px-4 text-right text-[14px] font-semibold whitespace-nowrap" :class="isCredit(txn) ? 'text-credit' : 'text-ink'">
            {{ isCredit(txn) ? '+' : '' }}{{ formatCurrency(txn.amount) }}
          </td>
          <td v-if="!readonly" class="py-3 pl-1 pr-3 text-right">
            <button
              type="button"
              class="ns-icon opacity-0 group-hover:opacity-100 focus-visible:opacity-100 text-ink-soft"
              aria-label="Edit transaction"
              title="Edit"
              @click.stop="emit('edit', txn)"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
          </td>
        </tr>
      </tbody>
      <tbody v-if="minRows">
        <template v-for="(_, i) in fillRows" :key="i">
          <tr>
            <td
              :colspan="colCount"
              class="py-3 h-[49px] text-center"
              :class="i === 0 && transactions.length === 0 ? 'text-ink-soft' : ''"
            >{{ i === 0 && transactions.length === 0 ? (emptyText || 'No transactions') : '' }}</td>
          </tr>
        </template>
      </tbody>
    </table>
    <div v-if="transactions.length === 0 && !minRows" class="text-center py-12 text-ink-soft">{{ emptyText || 'No transactions' }}</div>
  </div>
</template>