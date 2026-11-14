<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import AppInput from '@/components/AppInput.vue';
import AppSelect from '@/components/AppSelect.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { profile, apiKeys } from '@/lib/api/client';
import { API_BASE } from '@/lib/api/client';
import { useAuthStore } from '@/lib/stores/auth';
import { getAccessToken, setTokens } from '@/lib/api/transport';
import { applyTheme, current as currentTheme, type Theme } from '@/lib/theme';
import AppTabs from '@/components/ui/AppTabs.vue';

const themeTabs = [
  { key: 'system', label: 'System' },
  { key: 'light', label: 'Light' },
  { key: 'dark', label: 'Dark' }
];
import { Camera, Save, Loader2, ShieldCheck, Plus, Trash2 } from '@lucide/vue';
import { formatDate } from '@/lib/utils/format';

const auth = useAuthStore();

const loading = ref(true);
const error = ref('');
const notice = ref('');

const form = ref({ name: '', email: '', avatarUrl: '' });
const password = ref({ currentPassword: '', newPassword: '' });

// --- API keys ---
const keys = ref<any[]>([]);
const newKeyName = ref('');
const newKeyDays = ref(0);
const newKeyScope = ref('read');
const newKeyResult = ref('');
const keyNotice = ref('');
const keyError = ref('');
const savingKey = ref(false);

async function loadKeys() {
  keyError.value = '';
  try {
    keys.value = await apiKeys().listKeys({});
  } catch (e: any) {
    keyError.value = e?.message || 'Failed to load API keys';
  }
}

async function createKey() {
  savingKey.value = true;
  keyError.value = '';
  keyNotice.value = '';
  newKeyResult.value = '';
  try {
    const resp = await apiKeys().createKey({
      name: newKeyName.value,
      expiresInDays: newKeyDays.value,
      scope: newKeyScope.value
    });
    newKeyResult.value = resp?.key || '';
    newKeyName.value = '';
    newKeyDays.value = 0;
    newKeyScope.value = 'read';
    await loadKeys();
  } catch (e: any) {
    keyError.value = e?.message || 'Failed to create API key';
  } finally {
    savingKey.value = false;
  }
}

function maskKey(k: any): string {
  const p = k?.keyPrefix || '';
  return p ? p + '••••' : '••••';
}

async function revokeKey(id: string) {
  if (!confirm('Revoke this API key?')) return;
  keyError.value = '';
  try {
    await apiKeys().deleteKey({ id });
    await loadKeys();
  } catch (e: any) {
    keyError.value = e?.message || 'Failed to revoke API key';
  }
}
const savingProfile = ref(false);
const savingPassword = ref(false);
const loggingOutAll = ref(false);
const confirmLogoutAll = ref(false);
const avatarBusy = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);

const avatarSrc = computed(() =>
  form.value.avatarUrl ? API_BASE + form.value.avatarUrl : ''
);

async function loadProfile() {
  loading.value = true;
  error.value = '';
  try {
    const resp = await profile().getProfile({});
    form.value = { name: resp.name || '', email: resp.email || '', avatarUrl: resp.avatarUrl || '' };
  } catch (e: any) {
    error.value = e?.message || 'Failed to load profile';
  }
  loading.value = false;
}

async function saveProfile() {
  savingProfile.value = true;
  error.value = '';
  notice.value = '';
  try {
    const resp = await profile().updateProfile({ name: form.value.name, email: form.value.email });
    form.value.name = resp.name;
    form.value.email = resp.email;
    auth.setUser({ userId: auth.state.user?.userId || '', email: resp.email, name: resp.name });
    notice.value = 'Profile updated';
  } catch (e: any) {
    error.value = e?.message || 'Failed to update profile';
  }
  savingProfile.value = false;
}

async function changePassword() {
  if (password.value.newPassword.length < 6) {
    error.value = 'New password must be at least 6 characters';
    return;
  }
  savingPassword.value = true;
  error.value = '';
  notice.value = '';
  try {
    const resp = await profile().changePassword(password.value);
    // The API returns fresh tokens (the old refresh is revoked); store them
    // or the session dies at the next refresh.
    if (resp?.accessToken && resp?.refreshToken) {
      setTokens(resp.accessToken, resp.refreshToken);
    }
    password.value = { currentPassword: '', newPassword: '' };
    notice.value = 'Password changed. Other sessions were signed out.';
  } catch (e: any) {
    error.value = e?.message || 'Failed to change password';
  }
  savingPassword.value = false;
}

async function uploadAvatar(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  avatarBusy.value = true;
  error.value = '';
  notice.value = '';
  try {
    const fd = new FormData();
    fd.append('file', file);
    const resp = await fetch(API_BASE + '/api/me/avatar', {
      method: 'POST',
      headers: { Authorization: `Bearer ${getAccessToken()}` },
      body: fd
    });
    if (!resp.ok) {
      const body = await resp.json().catch(() => ({}));
      throw new Error(body.message || 'Avatar upload failed');
    }
    const data = await resp.json();
    form.value.avatarUrl = data.avatarUrl || '';
    notice.value = 'Avatar updated';
  } catch (err: any) {
    error.value = err?.message || 'Failed to upload avatar';
  }
  avatarBusy.value = false;
  if (fileInput.value) fileInput.value.value = '';
}

async function confirmLogoutAllNow() {
  loggingOutAll.value = true;
  error.value = '';
  notice.value = '';
  try {
    await profile().logoutAll({});
    notice.value = 'All other sessions signed out. You may need to log in again on other devices.';
  } catch (e: any) {
    error.value = e?.message || 'Failed to sign out all sessions';
  }
  loggingOutAll.value = false;
  confirmLogoutAll.value = false;
}

onMounted(() => {
  loadProfile();
  loadKeys();
});
</script>

<template>
  <div class="w-full space-y-5">
    <SectionHeader title="Settings" subtitle="Manage your profile, avatar, and password" />

    <div v-if="error" class="rounded-card border border-debit/40 px-3.5 py-2.5 text-[12.5px] text-debit">{{ error }}</div>
    <div v-if="notice" class="rounded-card border border-credit/40 px-3.5 py-2.5 text-[12.5px] text-credit">{{ notice }}</div>

    <div v-if="loading" class="py-16 text-center text-ink-mute">Loading…</div>

    <template v-else>
      <div class="overflow-hidden rounded-card border border-rule bg-page">
        <div class="divide-y divide-rule">

          <section class="flex flex-col gap-4 px-4 py-4">
            <h2 class="text-[15px] font-semibold text-ink">Profile</h2>

            <div class="flex items-center gap-4">
              <div class="relative">
                <div class="flex h-[68px] w-[68px] items-center justify-center overflow-hidden rounded-[6px] border border-rule bg-page-sunk text-[20px] font-medium text-ink-soft">
                  <img v-if="avatarSrc" :src="avatarSrc" alt="Avatar" class="w-full h-full object-cover" />
                  <span v-else>{{ (form.name || '?').slice(0, 2).toUpperCase() }}</span>
                </div>
                <button
                  class="absolute -bottom-1.5 -right-1.5 flex h-7 w-7 items-center justify-center rounded-full border border-rule bg-page text-ink-soft transition-colors hover:text-ink"
                  aria-label="Upload avatar"
                  :disabled="avatarBusy"
                  @click="fileInput?.click()"
                >
                  <Camera class="w-4 h-4" />
                </button>
                <input ref="fileInput" type="file" accept="image/png,image/jpeg,image/webp" class="hidden" @change="uploadAvatar" />
              </div>
              <div class="text-[12px] text-ink-mute">
                <Loader2 v-if="avatarBusy" class="w-4 h-4 animate-spin inline" />
                <span v-else>PNG, JPG or WebP, up to 5MB</span>
              </div>
            </div>

            <div class="grid gap-4 sm:grid-cols-2">
              <AppInput v-model="form.name" label="Name" placeholder="Your name" />
              <AppInput v-model="form.email" label="Email" type="email" placeholder="you@example.com" autocomplete="email" />
            </div>

            <div class="flex justify-end">
              <button class="ns-primary" :disabled="savingProfile" @click="saveProfile">
                <Loader2 v-if="savingProfile" class="w-4 h-4 animate-spin" />
                <Save v-else class="w-4 h-4" />
                Save
              </button>
            </div>
          </section>

          <section class="flex flex-col gap-4 px-4 py-4">
            <h2 class="text-[15px] font-semibold text-ink">Change password</h2>
            <p class="text-[12px] text-ink-mute">Changing your password signs out all other sessions.</p>
            <div class="grid gap-4 sm:grid-cols-2">
              <AppInput v-model="password.currentPassword" label="Current password" type="password" autocomplete="current-password" />
              <AppInput v-model="password.newPassword" label="New password" type="password" autocomplete="new-password" />
            </div>
            <div class="flex justify-end">
              <button class="ns-primary" :disabled="savingPassword" @click="changePassword">
                <Loader2 v-if="savingPassword" class="w-4 h-4 animate-spin" />
                <Save v-else class="w-4 h-4" />
                Update password
              </button>
            </div>
          </section>

          <section class="flex items-center justify-between gap-4 px-4 py-4">
            <div>
              <h2 class="text-[15px] font-semibold text-ink">Appearance</h2>
              <p class="text-[12px] text-ink-mute">Choose the app theme. Default follows your system settings.</p>
            </div>
            <AppTabs :items="themeTabs" :model-value="currentTheme" @update:model-value="applyTheme($event as Theme)" />
          </section>

          <section class="flex flex-col gap-4 px-4 py-4">
            <div>
              <h2 class="text-[15px] font-semibold text-ink">API keys</h2>
              <p class="text-[12px] text-ink-mute">Keys for external tools (e.g. MCP clients). Max 50 per user. Never shown again after creation.</p>
            </div>

            <p v-if="keyError" class="text-[12.5px] text-debit">{{ keyError }}</p>
            <p v-if="keyNotice" class="text-[12.5px] text-credit">{{ keyNotice }}</p>

            <div v-if="newKeyResult" class="rounded-card border border-accent/40 bg-accent-wash p-3">
              <p class="text-[12px] text-ink-mute">Copy this key now — it won't be shown again:</p>
              <code class="block break-all text-ink text-sm mt-1 font-mono">{{ newKeyResult }}</code>
            </div>

            <div class="grid items-end gap-3 md:grid-cols-[minmax(0,1fr)_140px_150px_auto]">
              <AppInput v-model="newKeyName" label="Name" placeholder="e.g. Claude Desktop" />
              <AppInput v-model="newKeyDays" label="Expires (days)" type="number" min="0" />
              <AppSelect v-model="newKeyScope" label="Scope">
                <option value="read">Read</option>
                <option value="read_write">Read + write</option>
              </AppSelect>
              <button class="ns-primary" :disabled="savingKey" @click="createKey">
                <Loader2 v-if="savingKey" class="w-4 h-4 animate-spin" />
                <Plus v-else class="w-4 h-4" />
                Create key
              </button>
            </div>

            <div v-if="keys.length" class="flex flex-col gap-2">
              <div v-for="k in keys" :key="k.id" class="flex items-center justify-between gap-4 rounded-card border border-rule p-3">
                <div>
                  <p class="text-ink text-sm font-medium">{{ k.name || 'Untitled' }}</p>
                  <p class="text-[12px] text-ink-mute font-mono">{{ maskKey(k) }}</p>
                  <p class="text-[11px] text-ink-mute">Created {{ formatDate(k.createdAt) }}<span v-if="k.expiresAt"> · expires {{ formatDate(k.expiresAt) }}</span></p>
                </div>
                <button class="inline-flex h-[30px] items-center justify-center gap-2 rounded-control border border-debit/30 px-3 text-[12.5px] font-medium text-debit transition-colors hover:bg-debit/10" @click="revokeKey(k.id)">
                  <Trash2 class="w-4 h-4" />
                  Revoke
                </button>
              </div>
            </div>
            <p v-else class="text-[12px] text-ink-mute">No API keys yet.</p>
          </section>

          <section class="flex items-center justify-between gap-4 px-4 py-4">
            <div>
              <h2 class="text-[15px] font-semibold text-ink">Sign out all sessions</h2>
              <p class="text-[12px] text-ink-mute">Revokes every login on other devices and browsers.</p>
            </div>
            <button class="inline-flex h-[30px] items-center justify-center gap-2 rounded-control border border-debit/30 px-3 text-[12.5px] font-medium text-debit transition-colors hover:bg-debit/10" :disabled="loggingOutAll" @click="confirmLogoutAll = true">
              <Loader2 v-if="loggingOutAll" class="w-4 h-4 animate-spin" />
              <ShieldCheck v-else class="w-4 h-4" />
              Sign out all
            </button>
          </section>

        </div>
      </div>
    </template>

    <ConfirmDialog
      v-if="confirmLogoutAll"
      title="Sign out all sessions?"
      message="Every other device will need to log in again. Continue?"
      @confirm="confirmLogoutAllNow"
      @cancel="confirmLogoutAll = false"
    />
  </div>
</template>
