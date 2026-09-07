<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import AppInput from '@/components/AppInput.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { profile, apiKeys } from '@/lib/api/client';
import { API_BASE } from '@/lib/api/client';
import { useAuthStore } from '@/lib/stores/auth';
import { getAccessToken, setTokens } from '@/lib/api/transport';
import { applyTheme, current as currentTheme } from '@/lib/theme';
import { Camera, Save, Loader2, ShieldCheck, Sun, Moon, Monitor, Plus, Trash2 } from '@lucide/vue';
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

    <div v-if="error" class="rounded-xl border border-expense/30 bg-expense/10 px-4 py-3 text-[13px] text-expense">{{ error }}</div>
    <div v-if="notice" class="rounded-xl border border-income/30 bg-income/10 px-4 py-3 text-[13px] text-income">{{ notice }}</div>

    <div v-if="loading" class="text-center py-16 text-subtle">Loading…</div>

    <template v-else>
      <div class="card bg-surface border border-border">
        <div class="card-body p-0 divide-y divide-border">

          <section class="p-6 space-y-5">
            <h2 class="text-lg font-semibold text-text">Profile</h2>

            <div class="flex items-center gap-4">
              <div class="relative">
                <div class="w-20 h-20 rounded-full bg-accent flex items-center justify-center text-2xl font-bold text-accent-content overflow-hidden">
                  <img v-if="avatarSrc" :src="avatarSrc" alt="Avatar" class="w-full h-full object-cover" />
                  <span v-else>{{ (form.name || '?').slice(0, 2).toUpperCase() }}</span>
                </div>
                <button
                  class="absolute -bottom-1 -right-1 btn btn-ghost btn-xs border border-track rounded-full p-1.5"
                  aria-label="Upload avatar"
                  :disabled="avatarBusy"
                  @click="fileInput?.click()"
                >
                  <Camera class="w-4 h-4" />
                </button>
                <input ref="fileInput" type="file" accept="image/png,image/jpeg,image/webp" class="hidden" @change="uploadAvatar" />
              </div>
              <div class="text-[12px] text-subtle">
                <Loader2 v-if="avatarBusy" class="w-4 h-4 animate-spin inline" />
                <span v-else>PNG, JPG or WebP, up to 5MB</span>
              </div>
            </div>

            <div class="grid gap-4 sm:grid-cols-2">
              <AppInput v-model="form.name" label="Name" placeholder="Your name" />
              <AppInput v-model="form.email" label="Email" type="email" placeholder="you@example.com" autocomplete="email" />
            </div>

            <div class="flex justify-end">
              <button class="btn btn-primary btn-sm gap-1.5" :disabled="savingProfile" @click="saveProfile">
                <Loader2 v-if="savingProfile" class="w-4 h-4 animate-spin" />
                <Save v-else class="w-4 h-4" />
                Save
              </button>
            </div>
          </section>

          <section class="p-6 space-y-5">
            <h2 class="text-lg font-semibold text-text">Change password</h2>
            <p class="text-[12px] text-subtle">Changing your password signs out all other sessions.</p>
            <div class="grid gap-4 sm:grid-cols-2">
              <AppInput v-model="password.currentPassword" label="Current password" type="password" autocomplete="current-password" />
              <AppInput v-model="password.newPassword" label="New password" type="password" autocomplete="new-password" />
            </div>
            <div class="flex justify-end">
              <button class="btn btn-primary btn-sm gap-1.5" :disabled="savingPassword" @click="changePassword">
                <Loader2 v-if="savingPassword" class="w-4 h-4 animate-spin" />
                <Save v-else class="w-4 h-4" />
                Update password
              </button>
            </div>
          </section>

          <section class="p-6 flex items-center justify-between gap-4">
            <div>
              <h2 class="text-lg font-semibold text-text">Appearance</h2>
              <p class="text-[12px] text-subtle">Choose the app theme. Default follows your system settings.</p>
            </div>
            <div class="flex gap-1 rounded-xl bg-surface border border-border p-1" role="group" aria-label="Theme">
              <button
                type="button"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[13px] font-medium transition-colors"
                :class="currentTheme === 'system' ? 'bg-primary-600 text-white' : 'text-text-muted hover:text-text'"
                @click="applyTheme('system')"
              >
                <Monitor class="w-4 h-4" stroke-width="1.5" />
                System
              </button>
              <button
                type="button"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[13px] font-medium transition-colors"
                :class="currentTheme === 'light' ? 'bg-primary-600 text-white' : 'text-text-muted hover:text-text'"
                @click="applyTheme('light')"
              >
                <Sun class="w-4 h-4" stroke-width="1.5" />
                Light
              </button>
              <button
                type="button"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[13px] font-medium transition-colors"
                :class="currentTheme === 'dark' ? 'bg-primary-600 text-white' : 'text-text-muted hover:text-text'"
                @click="applyTheme('dark')"
              >
                <Moon class="w-4 h-4" stroke-width="1.5" />
                Dark
              </button>
            </div>
          </section>

          <section class="p-6 space-y-5">
            <div>
              <h2 class="text-lg font-semibold text-text">API keys</h2>
              <p class="text-[12px] text-subtle">Keys for external tools (e.g. MCP clients). Max 50 per user. Never shown again after creation.</p>
            </div>

            <p v-if="keyError" class="text-error text-sm">{{ keyError }}</p>
            <p v-if="keyNotice" class="text-success text-sm">{{ keyNotice }}</p>

            <div v-if="newKeyResult" class="rounded-lg border border-primary/30 bg-base-200 p-3">
              <p class="text-[12px] text-subtle">Copy this key now — it won't be shown again:</p>
              <code class="block break-all text-text text-sm mt-1 font-mono">{{ newKeyResult }}</code>
            </div>

            <div class="flex items-end gap-3">
              <AppInput v-model="newKeyName" label="Name" placeholder="e.g. Claude Desktop" class="flex-1" />
              <AppInput v-model="newKeyDays" label="Expires (days, 0=never)" type="number" min="0" class="w-40" />
              <label class="form-control w-40">
                <span class="label-text text-[12px] text-subtle">Scope</span>
                <select v-model="newKeyScope" class="select select-bordered select-sm">
                  <option value="read">Read</option>
                  <option value="read_write">Read + write</option>
                </select>
              </label>
              <button class="btn btn-primary btn-sm gap-1.5" :disabled="savingKey" @click="createKey">
                <Loader2 v-if="savingKey" class="w-4 h-4 animate-spin" />
                <Plus v-else class="w-4 h-4" />
                Create key
              </button>
            </div>

            <div v-if="keys.length" class="space-y-2">
              <div v-for="k in keys" :key="k.id" class="flex items-center justify-between gap-4 rounded-lg border border-base-300 p-3">
                <div>
                  <p class="text-text text-sm font-medium">{{ k.name || 'Untitled' }}</p>
                  <p class="text-[12px] text-subtle font-mono">{{ maskKey(k) }}</p>
                  <p class="text-[11px] text-subtle">Created {{ formatDate(k.createdAt) }}<span v-if="k.expiresAt"> · expires {{ formatDate(k.expiresAt) }}</span></p>
                </div>
                <button class="btn btn-outline btn-error btn-sm gap-1.5" @click="revokeKey(k.id)">
                  <Trash2 class="w-4 h-4" />
                  Revoke
                </button>
              </div>
            </div>
            <p v-else class="text-[12px] text-subtle">No API keys yet.</p>
          </section>

          <section class="p-6 flex items-center justify-between gap-4">
            <div>
              <h2 class="text-lg font-semibold text-text">Sign out all sessions</h2>
              <p class="text-[12px] text-subtle">Revokes every login on other devices and browsers.</p>
            </div>
            <button class="btn btn-outline btn-error btn-sm gap-1.5" :disabled="loggingOutAll" @click="confirmLogoutAll = true">
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
