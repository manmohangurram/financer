<script setup lang="ts">
import { ref } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppInput from '@/components/AppInput.vue';
import { mail } from '@/lib/api/client';

const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>();

const form = ref({
  label: '',
  imapHost: '',
  imapPort: '993',
  username: '',
  password: '',
  senders: ''
});
const saving = ref(false);
const error = ref('');

function senders(): string[] {
  return form.value.senders
    .split(/[\s,;]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function validate(): string {
  if (!form.value.imapHost.trim()) return 'IMAP host is required';
  if (form.value.imapPort.trim() !== '993') return 'Only port 993 (implicit TLS) is supported';
  if (!form.value.username.trim()) return 'Username is required';
  if (!form.value.password) return 'An app password is required';
  const list = senders();
  if (!list.length) return 'List at least one sender address';
  const bad = list.find((s) => !s.includes('@'));
  if (bad) return `"${bad}" is not an email address`;
  return '';
}

async function submit() {
  error.value = validate();
  if (error.value) return;
  saving.value = true;
  try {
    await mail().createAccount({
      label: form.value.label.trim(),
      imapHost: form.value.imapHost.trim(),
      imapPort: 993,
      username: form.value.username.trim(),
      password: form.value.password,
      senderAllowlist: senders()
    });
    emit('saved');
    emit('close');
  } catch (e: any) {
    error.value = e?.message || 'Could not link the mailbox';
  }
  saving.value = false;
}
</script>

<template>
  <AppModal title="Link a mailbox" @close="emit('close')">
    <form class="px-6 py-5 space-y-3" @submit.prevent="submit">
      <p v-if="error" class="rounded-xl border border-expense/30 bg-expense/10 px-4 py-3 text-[13px] text-expense">{{ error }}</p>

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
        label="App password"
        type="password"
        autocomplete="current-password"
        hint="Gmail and others require an app password, not your account password"
        required
      />
      <AppInput
        v-model="form.senders"
        label="Alert senders"
        placeholder="alerts@hdfcbank.net, alerts@icicibank.com"
        hint="Only these senders are fetched. Comma or space separated"
        required
      />

      <div class="flex justify-end gap-2 pt-4">
        <button type="button" class="btn btn-ghost" @click="emit('close')">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? 'Linking…' : 'Link' }}</button>
      </div>
    </form>
  </AppModal>
</template>
