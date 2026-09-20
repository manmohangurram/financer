<script setup lang="ts">
import { ref } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppInput from '@/components/AppInput.vue';
import AppSelect from '@/components/AppSelect.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { accounts } from '@/lib/api/client';
import { ACCOUNT_TYPE_OPTIONS } from '@/lib/utils/accountType';

const props = defineProps<{ account: any | null }>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>();

const form = ref({
  bankName: props.account?.bankName || '',
  nickname: props.account?.nickname || '',
  endingNumbers: props.account?.endingNumbers || '',
  type: props.account?.type || 'CURRENT'
});
const saving = ref(false);
const error = ref('');
const endingError = ref('');
const confirmDelete = ref(false);

function validate() {
  endingError.value = '';
  const v = form.value.endingNumbers.trim();
  // Accounts created before this field existed must supply it before being edited.
  const required = !props.account || !props.account.endingNumbers;
  if (required && !v) {
    endingError.value = 'Last 4 digits are required';
    return false;
  }
  if (v && !/^\d{4}$/.test(v)) {
    endingError.value = 'Enter exactly 4 digits';
    return false;
  }
  return true;
}

async function submit() {
  error.value = '';
  if (!validate()) return;
  saving.value = true;
  try {
    if (props.account) await accounts().updateAccount({ id: props.account.id, ...form.value });
    else await accounts().createAccount(form.value);
    emit('saved');
    emit('close');
  } catch (e: any) { error.value = e?.message || 'Failed to save account'; }
  saving.value = false;
}

async function remove() {
  if (!props.account) return;
  try {
    await accounts().deleteAccount({ id: props.account.id });
    emit('saved');
    emit('close');
  } catch (e: any) { error.value = e?.message || 'Failed to delete account'; }
  confirmDelete.value = false;
}
</script>

<template>
  <AppModal :title="account ? 'Edit Account' : 'New Account'" @close="emit('close')">
    <form @submit.prevent="submit" class="px-6 py-5 space-y-3">
      <p v-if="error" class="rounded-xl border border-expense/30 bg-expense/10 px-4 py-3 text-[13px] text-expense">{{ error }}</p>
      <AppInput v-model="form.bankName" label="Bank Name" placeholder="e.g. Chase" />
      <AppInput v-model="form.nickname" label="Nickname" placeholder="e.g. Main Checking" />
      <AppInput
        v-model="form.endingNumbers"
        label="Account / Card Ending"
        placeholder="1234"
        hint="Last 4 digits, used to match bank alerts from email"
        :error="endingError"
        required
      />
      <AppSelect v-model="form.type" label="Account Type">
        <option v-for="opt in ACCOUNT_TYPE_OPTIONS" :key="opt.value" :value="opt.value">{{ opt.name }}</option>
      </AppSelect>
      <div class="flex items-center justify-between pt-6">
        <button v-if="account" type="button" @click="confirmDelete = true" class="btn btn-outline btn-error shrink-0">Delete</button>
        <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? 'Saving…' : account ? 'Update' : 'Create' }}</button>
      </div>
    </form>
  </AppModal>

  <ConfirmDialog
    v-if="confirmDelete"
    title="Delete account"
    :message="`Delete &quot;${form.nickname || form.bankName}&quot;? This can't be undone.`"
    @confirm="remove"
    @cancel="confirmDelete = false"
  />
</template>