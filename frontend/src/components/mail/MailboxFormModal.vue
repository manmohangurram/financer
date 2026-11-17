<script setup lang="ts">
import { computed, ref } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppInput from '@/components/AppInput.vue';
import { mail } from '@/lib/api/client';

interface Mailbox {
  id: string;
  label: string;
  imapHost: string;
  imapPort: number;
  username: string;
  enabled: boolean;
}

const props = defineProps<{ account?: Mailbox | null }>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>();

const editing = computed(() => !!props.account);
const form = ref({
  label: props.account?.label || '',
  imapHost: props.account?.imapHost || '',
  imapPort: String(props.account?.imapPort || 993),
  username: props.account?.username || '',
  password: '',
  enabled: props.account?.enabled ?? true
});
const saving = ref(false);
const error = ref('');

function validate(): string {
  if (!form.value.imapHost.trim()) return 'IMAP host is required';
  if (form.value.imapPort.trim() !== '993') return 'Only port 993 (implicit TLS) is supported';
  if (!form.value.username.trim()) return 'Username is required';
  if (!editing.value && !form.value.password) return 'An app password is required';
  return '';
}

async function submit() {
  error.value = validate();
  if (error.value) return;
  saving.value = true;
  try {
    if (editing.value && props.account) {
      await mail().updateAccount({
        id: props.account.id,
        label: form.value.label.trim(),
        imapHost: form.value.imapHost.trim(),
        imapPort: 993,
        username: form.value.username.trim(),
        password: form.value.password,
        enabled: form.value.enabled
      });
    } else {
      await mail().createAccount({
        label: form.value.label.trim(),
        imapHost: form.value.imapHost.trim(),
        imapPort: 993,
        username: form.value.username.trim(),
        password: form.value.password
      });
    }
    emit('saved');
    emit('close');
  } catch (e: any) {
    error.value = e?.message || (editing.value ? 'Could not update the mailbox' : 'Could not link the mailbox');
  }
  saving.value = false;
}
</script>

<template>
  <AppModal :title="editing ? 'Edit mailbox' : 'Link a mailbox'" @close="emit('close')">
    <form class="flex flex-col gap-3.5 px-5 py-4" @submit.prevent="submit">
      <p v-if="error" class="rounded-control border border-debit/30 bg-debit/5 px-3 py-2 text-[12.5px] text-debit">{{ error }}</p>

      <AppInput v-model="form.label" label="Label" placeholder="e.g. Personal Gmail" hint="Optional; shown in the list" />
      <AppInput v-model="form.imapHost" label="IMAP host" placeholder="imap.gmail.com" required />
      <AppInput
        v-model="form.username"
        label="Username"
        placeholder="you@gmail.com"
        autocomplete="username"
        required
      />
      <AppInput
        v-model="form.password"
        :label="editing ? 'New app password (optional)' : 'App password'"
        type="password"
        autocomplete="new-password"
        :hint="editing ? 'Leave empty to keep the saved password' : 'Gmail and others require an app password, not your account password'"
        :required="!editing"
      />
      <label v-if="editing" class="flex items-center gap-2 text-[13px] text-ink">
        <input v-model="form.enabled" type="checkbox" class="h-4 w-4 accent-[var(--accent)]" />
        Poll this mailbox
      </label>

      <div class="flex justify-end gap-2 pt-1">
        <button type="button" class="ns-quiet" @click="emit('close')">Cancel</button>
        <button type="submit" class="ns-primary" :disabled="saving">
          {{ saving ? 'Saving…' : editing ? 'Save mailbox' : 'Link mailbox' }}
        </button>
      </div>
    </form>
  </AppModal>
</template>
