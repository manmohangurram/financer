<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import SectionHeader from '@/components/accounts/SectionHeader.vue';
import AppInput from '@/components/AppInput.vue';
import AppSelect from '@/components/AppSelect.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import AppTabs from '@/components/ui/AppTabs.vue';
import MailboxFormModal from '@/components/mail/MailboxFormModal.vue';
import MailRulesTab from '@/components/mail/MailRulesTab.vue';
import { profile, apiKeys, llm, mail, API_BASE } from '@/lib/api/client';
import { useAuthStore } from '@/lib/stores/auth';
import { getAccessToken, setTokens } from '@/lib/api/transport';
import { applyTheme, current as currentTheme, type Theme } from '@/lib/theme';
import { Camera, Save, Loader2, ShieldCheck, Plus, Trash2, Pencil, RefreshCw, Mail } from '@lucide/vue';
import { formatDate } from '@/lib/utils/format';

interface Mailbox {
  id: string;
  label: string;
  imapHost: string;
  imapPort: number;
  username: string;
  enabled: boolean;
  lastPolledAt?: string | null;
  lastError: string;
}

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();

const TABS = [
  { key: 'account', label: 'Account' },
  { key: 'appearance', label: 'Appearance' },
  { key: 'security', label: 'Security' },
  { key: 'inbox', label: 'Inbox AI' },
  { key: 'mailboxes', label: 'Mailboxes' },
  { key: 'mail-rules', label: 'Mail rules' },
  { key: 'keys', label: 'API keys' }
] as const;
type TabKey = (typeof TABS)[number]['key'];

function tabFromRoute(): TabKey {
  const q = String(route.query.tab || '');
  return (TABS.some((t) => t.key === q) ? q : 'account') as TabKey;
}
const activeTab = ref<TabKey>(tabFromRoute());

function selectTab(key: string) {
  activeTab.value = key as TabKey;
  router.replace({ query: { ...route.query, tab: key } });
}

const themeTabs = [
  { key: 'system', label: 'System' },
  { key: 'light', label: 'Light' },
  { key: 'dark', label: 'Dark' }
];

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
const keyError = ref('');
const keyNotice = ref('');
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

// --- Inbox AI ---
const llmForm = ref({
  baseUrl: '',
  model: '',
  apiKey: '',
  timeoutSecs: 20,
  enabled: true,
  minSamples: 3,
  failureThreshold: 3,
  cooldownMinutes: 60,
  autoPollThrottleMinutes: 5
});
const apiKeySet = ref(false);
const llmUpdatedAt = ref('');
const llmError = ref('');
const llmNotice = ref('');
const savingLlm = ref(false);
const testingLlm = ref(false);

async function loadLlm() {
  llmError.value = '';
  try {
    const s = await llm().get({});
    llmForm.value = {
      baseUrl: s.baseUrl || '',
      model: s.model || '',
      apiKey: '',
      timeoutSecs: s.timeoutSecs || 20,
      enabled: s.enabled !== false,
      minSamples: s.minSamples || 3,
      failureThreshold: s.failureThreshold || 3,
      cooldownMinutes: s.cooldownMinutes || 60,
      autoPollThrottleMinutes: s.autoPollThrottleMinutes || 5
    };
    apiKeySet.value = !!s.apiKeySet;
    llmUpdatedAt.value = s.updatedAt || '';
  } catch (e: any) {
    llmError.value = e?.message || 'Failed to load Inbox AI settings';
  }
}

async function saveLlm() {
  savingLlm.value = true;
  llmError.value = '';
  llmNotice.value = '';
  try {
    const payload: any = { ...llmForm.value };
    if (!payload.apiKey) delete payload.apiKey;
    const s = await llm().update(payload);
    apiKeySet.value = !!s.apiKeySet;
    llmForm.value.apiKey = '';
    llmUpdatedAt.value = s.updatedAt || '';
    llmNotice.value = 'Inbox AI settings saved.';
  } catch (e: any) {
    llmError.value = e?.message || 'Failed to save Inbox AI settings';
  } finally {
    savingLlm.value = false;
  }
}

async function testLlm() {
  testingLlm.value = true;
  llmError.value = '';
  llmNotice.value = '';
  try {
    const res = await llm().test({});
    llmNotice.value = res?.message || 'Connection ok.';
  } catch (e: any) {
    llmError.value = e?.message || 'Could not reach the model';
  } finally {
    testingLlm.value = false;
  }
}

// --- Mailboxes ---
const mailboxes = ref<Mailbox[]>([]);
const mailboxError = ref('');
const mailboxNotice = ref('');
const showMailboxForm = ref(false);
const editMailbox = ref<Mailbox | null>(null);
const deleteMailboxId = ref('');
const pollingMailbox = ref('');

async function loadMailboxes() {
  mailboxError.value = '';
  try {
    const res = await mail().listAccounts({});
    mailboxes.value = res.accounts || [];
  } catch (e: any) {
    mailboxError.value = e?.message || 'Failed to load mailboxes';
  }
}

function addMailbox() {
  editMailbox.value = null;
  showMailboxForm.value = true;
}

function editMailboxOpen(m: Mailbox) {
  editMailbox.value = m;
  showMailboxForm.value = true;
}

async function pollMailbox(id: string) {
  pollingMailbox.value = id;
  mailboxError.value = '';
  mailboxNotice.value = '';
  try {
    const res = await mail().pollAccount({ id });
    mailboxNotice.value = `Checked: ${res.matched} matched, ${res.stored} stored, ${res.skipped} skipped.`;
    await loadMailboxes();
  } catch (e: any) {
    mailboxError.value = e?.message || 'Poll failed';
  } finally {
    pollingMailbox.value = '';
  }
}

async function confirmDeleteMailbox() {
  if (!deleteMailboxId.value) return;
  try {
    await mail().deleteAccount({ id: deleteMailboxId.value });
    mailboxNotice.value = 'Mailbox removed.';
    await loadMailboxes();
  } catch (e: any) {
    mailboxError.value = e?.message || 'Failed to remove the mailbox';
  }
  deleteMailboxId.value = '';
}

// --- Account/security ---
const savingProfile = ref(false);
const savingPassword = ref(false);
const loggingOutAll = ref(false);
const confirmLogoutAll = ref(false);
const avatarBusy = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);

const avatarSrc = computed(() => (form.value.avatarUrl ? API_BASE + form.value.avatarUrl : ''));

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
    if (resp?.accessToken && resp?.refreshToken) setTokens(resp.accessToken, resp.refreshToken);
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
    notice.value = 'All other sessions signed out.';
  } catch (e: any) {
    error.value = e?.message || 'Failed to sign out all sessions';
  }
  loggingOutAll.value = false;
  confirmLogoutAll.value = false;
}

onMounted(() => {
  loadProfile();
  loadKeys();
  loadLlm();
  loadMailboxes();
});
</script>

<template>
  <div class="w-full space-y-4">
    <SectionHeader title="Settings" subtitle="Account, security, Inbox AI, mailboxes and keys" />

    <AppTabs :items="TABS.map((t) => ({ ...t }))" :model-value="activeTab" @update:model-value="selectTab($event as string)" />

    <div v-if="error" class="rounded-card border border-debit/40 px-3.5 py-2.5 text-[12.5px] text-debit">{{ error }}</div>
    <div v-if="notice" class="rounded-card border border-credit/40 px-3.5 py-2.5 text-[12.5px] text-credit">{{ notice }}</div>

    <div v-if="loading && activeTab === 'account'" class="py-16 text-center text-ink-mute">Loading…</div>

    <template v-else>
      <!-- Account -->
      <section v-if="activeTab === 'account'" class="overflow-hidden rounded-card border border-rule bg-page">
        <div class="flex flex-col gap-4 px-4 py-4">
          <h2 class="text-[15px] font-semibold text-ink">Profile</h2>
          <div class="flex items-center gap-4">
            <div class="relative">
              <div class="flex h-[68px] w-[68px] items-center justify-center overflow-hidden rounded-[6px] border border-rule bg-page-sunk text-[20px] font-medium text-ink-soft">
                <img v-if="avatarSrc" :src="avatarSrc" alt="Avatar" class="w-full h-full object-cover" />
                <span v-else>{{ (form.name || '?').slice(0, 2).toUpperCase() }}</span>
              </div>
              <button
                class="absolute -bottom-1.5 -right-1.5 flex h-7 w-7 items-center justify-center rounded-full border border-rule bg-page text-ink-soft transition-colors hover:text-ink"
                aria-label="Upload avatar" :disabled="avatarBusy" @click="fileInput?.click()"
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
        </div>
      </section>

      <!-- Appearance -->
      <section v-else-if="activeTab === 'appearance'" class="flex items-center justify-between gap-4 rounded-card border border-rule bg-page px-4 py-4">
        <div>
          <h2 class="text-[15px] font-semibold text-ink">Appearance</h2>
          <p class="text-[12px] text-ink-mute">Choose the app theme. System follows your OS setting.</p>
        </div>
        <AppTabs :items="themeTabs" :model-value="currentTheme" @update:model-value="applyTheme($event as Theme)" />
      </section>

      <!-- Security -->
      <section v-else-if="activeTab === 'security'" class="overflow-hidden rounded-card border border-rule bg-page">
        <div class="flex flex-col gap-4 px-4 py-4">
          <div>
            <h2 class="text-[15px] font-semibold text-ink">Change password</h2>
            <p class="text-[12px] text-ink-mute">Changing your password signs out all other sessions.</p>
          </div>
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
        </div>
        <div class="flex items-center justify-between gap-4 border-t border-rule px-4 py-4">
          <div>
            <h2 class="text-[15px] font-semibold text-ink">Sign out all sessions</h2>
            <p class="text-[12px] text-ink-mute">Revokes every login on other devices and browsers.</p>
          </div>
          <button class="inline-flex h-[30px] items-center justify-center gap-2 rounded-control border border-debit/30 px-3 text-[12.5px] font-medium text-debit transition-colors hover:bg-debit/10" :disabled="loggingOutAll" @click="confirmLogoutAll = true">
            <Loader2 v-if="loggingOutAll" class="w-4 h-4 animate-spin" />
            <ShieldCheck v-else class="w-4 h-4" />
            Sign out all
          </button>
        </div>
      </section>

      <!-- Inbox AI -->
      <section v-else-if="activeTab === 'inbox'" class="overflow-hidden rounded-card border border-rule bg-page">
        <div class="flex flex-col gap-4 px-4 py-4">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h2 class="text-[15px] font-semibold text-ink">Inbox AI</h2>
              <p class="text-[12px] text-ink-mute">
                Any OpenAI-compatible chat endpoint (LiteLLM, DeepSeek, OpenAI, Ollama…). Used only to generate parsing
                templates when an email cannot be parsed.
              </p>
            </div>
            <label class="flex items-center gap-2 text-[12.5px] text-ink">
              <input v-model="llmForm.enabled" type="checkbox" class="h-4 w-4 accent-[var(--accent)]" />
              Enabled
            </label>
          </div>

          <p v-if="llmError" class="text-[12.5px] text-debit">{{ llmError }}</p>
          <p v-if="llmNotice" class="text-[12.5px] text-credit">{{ llmNotice }}</p>

          <div class="grid gap-4 sm:grid-cols-2">
            <AppInput
              v-model="llmForm.baseUrl"
              label="API base URL"
              placeholder="https://api.deepseek.com/v1"
              hint="OpenAI-compatible; /chat/completions is appended"
            />
            <AppInput v-model="llmForm.model" label="Model" placeholder="e.g. deepseek-chat" />
          </div>
          <AppInput
            v-model="llmForm.apiKey"
            label="API key"
            type="password"
            autocomplete="new-password"
            :hint="apiKeySet ? 'A key is stored. Leave empty to keep it.' : 'Leave empty if the endpoint needs no key'"
          />

          <details class="rounded-control border border-rule-soft px-3 py-2">
            <summary class="cursor-pointer text-[12.5px] text-ink-soft">Advanced</summary>
            <div class="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <AppInput v-model.number="llmForm.timeoutSecs" label="Timeout (s)" type="number" min="5" max="120" />
              <AppInput v-model.number="llmForm.minSamples" label="Samples to activate" type="number" min="1" max="10" />
              <AppInput v-model.number="llmForm.cooldownMinutes" label="Cooldown (min)" type="number" min="5" max="1440" />
              <AppInput v-model.number="llmForm.autoPollThrottleMinutes" label="Poll throttle (min)" type="number" min="1" max="60" />
            </div>
            <div class="mt-3 max-w-[220px]">
              <AppInput v-model.number="llmForm.failureThreshold" label="Failures before rebuild" type="number" min="1" max="10" />
            </div>
          </details>

          <p v-if="llmUpdatedAt" class="text-[11px] text-ink-faint">Last saved {{ formatDate(llmUpdatedAt) }}</p>

          <div class="flex justify-end gap-2">
            <button class="ns-quiet" :disabled="testingLlm" @click="testLlm">
              <Loader2 v-if="testingLlm" class="w-4 h-4 animate-spin" /> Test connection
            </button>
            <button class="ns-primary" :disabled="savingLlm" @click="saveLlm">
              <Loader2 v-if="savingLlm" class="w-4 h-4 animate-spin" />
              <Save v-else class="w-4 h-4" />
              Save
            </button>
          </div>
        </div>
      </section>

      <!-- Mailboxes -->
      <section v-else-if="activeTab === 'mailboxes'" class="overflow-hidden rounded-card border border-rule bg-page">
        <div class="flex items-center justify-between gap-4 px-4 py-4">
          <div>
            <h2 class="text-[15px] font-semibold text-ink">Mailboxes</h2>
            <p class="text-[12px] text-ink-mute">Linked IMAP accounts. Rules decide which mail is ignored, reviewed or auto-created.</p>
          </div>
          <button class="ns-primary" @click="addMailbox"><Plus class="w-4 h-4" /> Add mailbox</button>
        </div>

        <p v-if="mailboxError" class="px-4 pb-2 text-[12.5px] text-debit">{{ mailboxError }}</p>
        <p v-if="mailboxNotice" class="px-4 pb-2 text-[12.5px] text-credit">{{ mailboxNotice }}</p>

        <div v-if="mailboxes.length" class="divide-y divide-rule border-t border-rule">
          <div v-for="m in mailboxes" :key="m.id" class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
            <div class="min-w-0">
              <p class="flex items-center gap-2 text-[13px] text-ink">
                <Mail class="h-4 w-4 text-ink-mute" />
                {{ m.label || m.username }}
                <span v-if="!m.enabled" class="rounded border border-rule px-1.5 text-[10px] uppercase tracking-wide text-ink-mute">paused</span>
              </p>
              <p class="text-[11.5px] text-ink-mute">{{ m.username }} · {{ m.imapHost }}:{{ m.imapPort }}</p>
              <p class="text-[11px] text-ink-faint">
                Last polled {{ m.lastPolledAt ? formatDate(m.lastPolledAt) : 'never' }}
                <span v-if="m.lastError" class="text-debit"> · {{ m.lastError }}</span>
              </p>
            </div>
            <div class="flex items-center gap-1">
              <button class="ns-quiet text-[12.5px]" :disabled="pollingMailbox === m.id" @click="pollMailbox(m.id)">
                <Loader2 v-if="pollingMailbox === m.id" class="h-4 w-4 animate-spin" />
                <RefreshCw v-else class="h-4 w-4" /> Check
              </button>
              <button class="ns-quiet text-[12.5px]" @click="editMailboxOpen(m)"><Pencil class="h-4 w-4" /> Edit</button>
              <button class="ns-quiet text-[12.5px] text-debit" @click="deleteMailboxId = m.id"><Trash2 class="h-4 w-4" /> Remove</button>
            </div>
          </div>
        </div>
        <p v-else class="border-t border-rule px-4 py-8 text-center text-[13px] text-ink-mute">
          No mailboxes linked. Add one to start receiving alerts.
        </p>
      </section>

      <!-- Mail rules -->
      <MailRulesTab v-else-if="activeTab === 'mail-rules'" />

      <!-- API keys -->
      <section v-else-if="activeTab === 'keys'" class="overflow-hidden rounded-card border border-rule bg-page">
        <div class="flex flex-col gap-4 px-4 py-4">
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
        </div>
      </section>
    </template>

    <MailboxFormModal
      v-if="showMailboxForm"
      :account="editMailbox"
      @close="showMailboxForm = false"
      @saved="loadMailboxes"
    />

    <ConfirmDialog
      v-if="deleteMailboxId"
      title="Remove this mailbox?"
      message="Stored mail and learned templates stay, but no new mail will be fetched from it."
      @confirm="confirmDeleteMailbox"
      @cancel="deleteMailboxId = ''"
    />
    <ConfirmDialog
      v-if="confirmLogoutAll"
      title="Sign out all sessions?"
      message="Every other device will need to log in again. Continue?"
      @confirm="confirmLogoutAllNow"
      @cancel="confirmLogoutAll = false"
    />
  </div>
</template>
