<script setup lang="ts">
import { ref } from 'vue';
import AppModal from '@/components/AppModal.vue';
import TransactionForm from '@/components/workspace/TransactionForm.vue';
import RuleForm from '@/components/workspace/RuleForm.vue';
import CategoryForm from '@/components/workspace/CategoryForm.vue';
import { dateToUnixSeconds, toLocalDateString } from '@/lib/utils/format';
import { roundMoney } from '@/lib/utils/money';
import { emptyOutput, fromRuleAction, toRuleAction, type RuleOutput } from '@/lib/utils/ruleOutputs';

type ModalType = 'transaction' | 'rule' | 'category';

const props = defineProps<{
  type: ModalType;
  editingItem: any | null;
  accounts: any[];
  categories: any[];
  defaultAccountId: string;
}>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'submit', payload: any): void; (e: 'delete'): void; (e: 'unlink'): void }>();

const txnForm = ref(
  props.type === 'transaction' && props.editingItem
    ? {
        name: props.editingItem.name,
        amount: props.editingItem.amount,
        type: props.editingItem.type,
        accountId: props.editingItem.accountId,
        occurredAt: toLocalDateString(new Date(props.editingItem.occurredAt || Date.now())),
        categoryIds: props.editingItem.categoryIds || []
      }
    : { name: '', amount: 0, type: 'DEBIT', accountId: props.defaultAccountId, occurredAt: toLocalDateString(new Date()), categoryIds: [] as string[] }
);

const ruleForm = ref(
  props.type === 'rule' && props.editingItem
    ? {
        name: props.editingItem.name,
        priority: props.editingItem.priority || 0,
        logic: props.editingItem.logic || 'OR',
        conditions: props.editingItem.conditions?.length ? props.editingItem.conditions : [{ matchField: 'NAME', operator: 'CONTAINS', pattern: '' }],
        outputs: (props.editingItem.actions || []).map(fromRuleAction).filter(Boolean) as RuleOutput[]
      }
    : { name: '', priority: 0, logic: 'OR', conditions: [{ matchField: 'NAME', operator: 'CONTAINS', pattern: '' }], outputs: [emptyOutput()] }
);

const catForm = ref({ name: props.type === 'category' && props.editingItem ? props.editingItem.name : '' });

const titles: Record<ModalType, string> = { transaction: 'Transaction', rule: 'Rule', category: 'Category' };

function handleSubmit() {
  if (props.type === 'transaction') {
    emit('submit', { ...txnForm.value, amount: roundMoney(txnForm.value.amount), occurredAt: { seconds: dateToUnixSeconds(txnForm.value.occurredAt), nanos: 0 } });
  } else if (props.type === 'rule') {
    emit('submit', { ...ruleForm.value, actions: ruleForm.value.outputs.map(toRuleAction) });
  } else {
    emit('submit', { ...catForm.value });
  }
}
</script>

<template>
  <AppModal :title="(editingItem ? 'Edit ' : 'New ') + titles[type]" :xwide="type === 'rule'" @close="emit('close')">
    <form @submit.prevent="handleSubmit" class="p-6 space-y-4">
      <TransactionForm v-if="type === 'transaction'" v-model="txnForm" :accounts="accounts" :categories="categories" />
      <RuleForm v-else-if="type === 'rule'" v-model="ruleForm" :categories="categories" :accounts="accounts" />
      <CategoryForm v-else v-model="catForm" />

      <div class="flex items-center justify-between pt-2">
        <div class="flex items-center gap-2">
          <button
            v-if="type === 'transaction' && editingItem?.linkedTransferId"
            type="button"
            class="btn btn-outline btn-error shrink-0"
            @click="emit('unlink')"
          >Unlink transfer</button>
          <button v-if="editingItem" type="button" @click="emit('delete')" class="btn btn-outline btn-error shrink-0">Delete</button>
        </div>
        <button type="submit" class="btn btn-primary">{{ editingItem ? 'Update' : 'Create' }}</button>
      </div>
    </form>
  </AppModal>
</template>
