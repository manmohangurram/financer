<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { Plus } from '@lucide/vue';
import { accounts, categories, mail } from '@/lib/api/client';
import MailRuleFormModal from '@/components/mail/MailRuleFormModal.vue';

const rules = ref<any[]>([]);
const accountList = ref<any[]>([]);
const categoryList = ref<any[]>([]);
const loading = ref(true);
const error = ref('');
const showForm = ref(false);
const editing = ref<any | null>(null);

const sorted = computed(() => [...rules.value].sort((a, b) => (a.position || 0) - (b.position || 0)));

function actionTone(action: string): string {
  if (action === 'AUTO') return 'badge-success';
  if (action === 'IGNORE') return 'badge-ghost';
  return 'badge-warning';
}

async function load() {
  loading.value = true;
  error.value = '';
  try {
    const [ruleResp, accResp, catResp] = await Promise.all([
      mail().listRules({}),
      accounts().listAccounts({}),
      categories().listCategories({ pageSize: 1000 })
    ]);
    rules.value = ruleResp.rules || [];
    accountList.value = accResp.accounts || [];
    categoryList.value = catResp.categories || [];
  } catch (e: any) {
    error.value = e?.message || 'Failed to load mail rules';
  }
  loading.value = false;
}

function openCreate() {
  editing.value = null;
  showForm.value = true;
}

function openEdit(rule: any) {
  editing.value = rule;
  showForm.value = true;
}

onMounted(load);
</script>

<template>
  <section class="overflow-hidden rounded-card border border-rule bg-page">
    <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-4">
      <div>
        <h2 class="text-[15px] font-semibold text-ink">Mail rules</h2>
        <p class="text-[12px] text-ink-mute">
          Sender, subject and body conditions decide which mail is ignored, reviewed or auto-booked. First match wins.
          A rule can carry a manual extraction template for its sender.
        </p>
      </div>
      <button class="btn btn-sm btn-primary" @click="openCreate"><Plus class="h-4 w-4" /> Add rule</button>
    </div>

    <p v-if="error" role="alert" class="px-4 pb-2 text-[12.5px] text-debit">{{ error }}</p>

    <div v-if="loading" class="border-t border-rule px-4 py-6 text-[13px] text-ink-mute">Loading rules…</div>

    <div v-else-if="!sorted.length" class="border-t border-rule px-4 py-8 text-center">
      <p class="text-[13px] text-ink-mute">No mail rules yet.</p>
      <p class="mt-1 text-[12px] text-ink-faint">Without rules, parsed mail waits for review.</p>
    </div>

    <div v-else class="border-t border-rule">
      <div class="grid grid-cols-[40px_minmax(0,1fr)_minmax(0,1fr)_120px] gap-3 bg-page-sunk px-4 py-2 text-label font-medium uppercase tracking-[0.06em] text-ink-mute">
        <span class="text-center">#</span>
        <span>From</span>
        <span>Subject</span>
        <span>Action</span>
      </div>
      <button
        v-for="rule in sorted"
        :key="rule.id"
        type="button"
        class="grid w-full grid-cols-[40px_minmax(0,1fr)_minmax(0,1fr)_120px] items-center gap-3 border-b border-rule-soft px-4 py-2.5 text-left last:border-0 hover:bg-page-sunk/60 focus-visible:bg-page-sunk/60 focus-visible:outline-none"
        @click="openEdit(rule)"
      >
        <span class="text-center text-[11px] text-ink-mute">{{ rule.position || '—' }}</span>
        <span class="truncate text-[12.5px] text-ink">{{ rule.fromPattern || '—' }}</span>
        <span class="truncate text-[12.5px] text-ink-soft">
          <template v-if="rule.subjectContains">{{ rule.subjectContains }}</template>
          <template v-else-if="rule.bodyContains">body: {{ rule.bodyContains }}</template>
          <template v-else>—</template>
        </span>
        <span class="flex items-center gap-2">
          <span class="badge badge-sm badge-outline" :class="actionTone(rule.action)">{{ rule.action }}</span>
          <span v-if="rule.template" class="stamp text-ink-mute" title="Manual extraction template">template</span>
          <span v-if="!rule.enabled" class="text-[10px] uppercase tracking-wide text-ink-faint">paused</span>
        </span>
      </button>
    </div>
  </section>

  <MailRuleFormModal v-if="showForm" :rule="editing" :accounts="accountList" :categories="categoryList" @close="showForm = false" @saved="load" />
</template>
