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
  type: props.account?.type || 'CURRENT'
});
const saving = ref(false);
const confirmDelete = ref(false);

async function submit() {
  saving.value = true;
  try {
    if (props.account) await accounts().updateAccount({ id: props.account.id, ...form.value });
    else await accounts().createAccount(form.value);
    emit('saved');
    emit('close');
  } catch (e) { console.error(e); }
  saving.value = false;
}

async function remove() {
  if (!props.account) return;
  try {
    await accounts().deleteAccount({ id: props.account.id });
    emit('saved');
    emit('close');
  } catch (e) { console.error(e); }
  confirmDelete.value = false;
}
</script>

<template>
  <AppModal :title="account ? 'Edit Account' : 'New Account'" @close="emit('close')">
    <form @submit.prevent="submit" class="px-6 py-5 space-y-3">
      <AppInput v-model="form.bankName" label="Bank Name" placeholder="e.g. Chase" />
      <AppInput v-model="form.nickname" label="Nickname" placeholder="e.g. Main Checking" />
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