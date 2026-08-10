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
  return { name: other.name, account: acc ? acc.accountNickname || acc.bankName : '' };
}
</script>

<template>
  <div class="w-full" :class="scrollable ? 'max-h-56 overflow-auto' : 'overflow-x-auto'">
    <table class="w-full">
      <thead>
        <tr class="text-[12px] text-subtle font-medium border-b border-border group" :class="scrollable && 'sticky top-0 bg-surface z-10'">
          <th v-if="!readonly" class="py-3 pl-4 w-8">
            <input
              type="checkbox"
              class="checkbox checkbox-sm transition-opacity"
              :class="hasSelection ? 'opacity-100' : 'opacity-0 group-hover:opacity-100 focus-visible:opacity-100'"
              :checked="allSelected"
              :indeterminate="someSelected"
              @change="emit('toggle-select-all')"
              aria-label="Select all"
            />
          </th>
          <th class="text-left py-3 px-2 w-[20%] lg:w-[10%]">Date</th>
          <th class="text-left py-3 px-2 w-[50%] lg:w-[40%]">Name</th>
          <th class="text-left py-3 px-2 hidden lg:table-cell lg:w-[30%]">Categories</th>
          <th class="text-right py-3 px-4" :class="readonly ? 'w-[30%] lg:w-[20%]' : 'w-[20%] lg:w-[15%]'">Amount</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-border/60">
        <tr
          v-for="txn in transactions"
          :key="txn.id"
          class="transition-colors group"
          :class="[
            !readonly && 'cursor-pointer',
            selectedIds?.includes(txn.id) ? 'bg-primary-500/10 hover:bg-primary-500/15' : 'hover:bg-white/[0.015]'
          ]"
          @click="openRow(txn)"
        >
          <td v-if="!readonly" class="py-3 pl-4 w-8" @click.stop>
            <input
              type="checkbox"
              class="checkbox checkbox-sm transition-opacity"
              :class="hasSelection ? 'opacity-100' : 'opacity-0 group-hover:opacity-100 focus-visible:opacity-100'"
              :checked="selectedIds?.includes(txn.id)"
              @change="emit('toggle-select', txn)"
              aria-label="Select row"
            />
          </td>
          <td class="py-3 px-2 text-[13px] text-subtle whitespace-nowrap">{{ formatDate(txn.occurredAt || '') }}</td>
          <td class="py-3 px-2">
            <div class="flex items-center gap-3">
              <div class="w-2 h-2 rounded-full shrink-0" :class="isCredit(txn) ? 'bg-income' : 'bg-expense'"></div>
              <div class="min-w-0">
                <div class="flex items-center gap-1.5">
                  <span class="text-[14px] text-text font-medium truncate">{{ txn.name }}</span>
                  <button
                    v-if="!readonly && txn.type === 'DEBIT' && !txn.linkedTransferId"
                    type="button"
                    class="opacity-0 group-hover:opacity-100 focus-visible:opacity-100 text-subtle hover:text-primary-400 shrink-0 transition-opacity"
                    aria-label="Transfer this transaction"
                    title="Transfer to another account"
                    @click.stop="emit('transfer', txn)"
                  >
                    <ArrowLeftRight class="w-3.5 h-3.5" />
                  </button>
                  <button
                    v-if="txn.linkedTransferId"
                    type="button"
                    class="text-subtle hover:text-primary-400 shrink-0"
                    aria-label="Show linked transfer"
                    @click="revealed = revealed === txn.id ? null : txn.id"
                  >
                    <ArrowLeftRight class="w-3.5 h-3.5" />
                  </button>
                </div>
                <div v-if="revealed === txn.id && counterpart(txn)" class="text-[11px] text-subtle mt-0.5 whitespace-nowrap">
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
          <td class="py-3 px-4 text-right text-[14px] font-semibold whitespace-nowrap" :class="isCredit(txn) ? 'text-income' : 'text-text'">
            {{ formatCurrency(txn.amount) }}
          </td>
          <td v-if="!readonly" class="py-3 pl-1 pr-3 text-right">
            <button
              type="button"
              class="opacity-0 group-hover:opacity-100 focus-visible:opacity-100 text-subtle hover:text-primary-400 shrink-0 transition-opacity"
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
              :class="i === 0 && transactions.length === 0 ? 'text-subtle' : ''"
            >{{ i === 0 && transactions.length === 0 ? (emptyText || 'No transactions') : '' }}</td>
          </tr>
        </template>
      </tbody>
    </table>
    <div v-if="transactions.length === 0 && !minRows" class="text-center py-12 text-subtle">{{ emptyText || 'No transactions' }}</div>
  </div>
</template>