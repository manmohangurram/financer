<script setup lang="ts">
import { ref } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppInput from '@/components/AppInput.vue';
import MailTemplateEditor from '@/components/mail/MailTemplateEditor.vue';
import { mail } from '@/lib/api/client';
import { cleanedTemplate, templateProblem, type TemplatePatterns } from '@/lib/mailTemplate';

const props = defineProps<{
  rule?: any | null;
  accounts?: any[];
  categories?: any[];
}>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>();

const editing = !!props.rule;
const form = ref({
  fromPattern: props.rule?.fromPattern || '',
  subjectContains: props.rule?.subjectContains || '',
  bodyContains: props.rule?.bodyContains || '',
  action: props.rule?.action || 'REVIEW',
  enabled: props.rule?.enabled ?? true,
  accountId: props.rule?.accountId || '',
  categoryIds: [...(props.rule?.categoryIds || [])] as string[],
  template: (props.rule?.template ? JSON.parse(JSON.stringify(props.rule.template)) : null) as TemplatePatterns | null
});
const saving = ref(false);
const confirmingDelete = ref(false);
const error = ref('');

function toggleCategory(id: string) {
  form.value.categoryIds = form.value.categoryIds.includes(id)
    ? form.value.categoryIds.filter((x) => x !== id)
    : [...form.value.categoryIds, id];
}

async function submit() {
  error.value = '';
  if (!form.value.fromPattern.trim() && !form.value.subjectContains.trim() && !form.value.bodyContains.trim()) {
    error.value = 'Add at least one condition: sender, subject or body.';
    return;
  }
  if (form.value.template) {
    if (!form.value.fromPattern.trim()) {
      error.value = 'A template needs a From condition.';
      return;
    }
    const problem = templateProblem(form.value.template);
    if (problem) {
      error.value = problem;
      return;
    }
  }
  saving.value = true;
  try {
    const payload = {
      position: props.rule?.position || 0,
      enabled: form.value.enabled,
      fromPattern: form.value.fromPattern.trim(),
      subjectContains: form.value.subjectContains.trim(),
      bodyContains: form.value.bodyContains.trim(),
      action: form.value.action,
      accountId: form.value.accountId || null,
      categoryIds: form.value.categoryIds,
      template: form.value.template ? cleanedTemplate(form.value.template) : null
    };
    if (editing) await mail().updateRule({ id: props.rule.id, ...payload });
    else await mail().createRule(payload);
    emit('saved');
    emit('close');
  } catch (e: any) {
    error.value = e?.message || 'Could not save the rule';
  }
  saving.value = false;
}

async function remove() {
  if (!props.rule) return;
  if (!confirmingDelete.value) {
    confirmingDelete.value = true;
    return;
  }
  saving.value = true;
  try {
    await mail().deleteRule({ id: props.rule.id });
    emit('saved');
    emit('close');
  } catch (e: any) {
    error.value = e?.message || 'Could not delete the rule';
  }
  saving.value = false;
}
</script>

<template>
  <AppModal :title="editing ? 'Edit mail rule' : 'New mail rule'" wide @close="emit('close')">
    <form class="flex flex-col gap-4 px-5 py-4" @submit.prevent="submit">
      <p v-if="error" role="alert" class="rounded-control border border-debit/30 bg-debit/5 px-3 py-2 text-[12.5px] text-debit">{{ error }}</p>
      <p class="text-[12.5px] text-ink-mute">
        Every condition is a case-insensitive contains check; leave one empty to ignore it. The first matching rule wins.
      </p>

      <AppInput v-model="form.fromPattern" label="From contains" placeholder="alerts@hdfcbank.net" />
      <AppInput v-model="form.subjectContains" label="Subject contains" placeholder="Debit Alert" />
      <AppInput v-model="form.bodyContains" label="Body contains" placeholder="debited" />

      <MailTemplateEditor v-model="form.template" :from-pattern="form.fromPattern" />

      <label class="flex flex-col gap-1.5">
        <span class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Book to account (optional)</span>
        <select v-model="form.accountId" class="select select-bordered w-full text-[13.5px]">
          <option value="">No fixed account</option>
          <option v-for="a in accounts || []" :key="a.id" :value="a.id">{{ a.nickname || a.bankName }}</option>
        </select>
        <span class="text-[11.5px] text-ink-mute">AUTO uses this account when the mail has no ending digits.</span>
      </label>

      <div v-if="(categories || []).length" class="flex flex-col gap-1.5">
        <span class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Categories (optional)</span>
        <div class="flex flex-wrap gap-1.5">
          <button
            v-for="c in categories"
            :key="c.id"
            type="button"
            class="btn btn-xs"
            :class="form.categoryIds.includes(c.id) ? 'btn-primary' : 'btn-outline'"
            :aria-pressed="form.categoryIds.includes(c.id)"
            @click="toggleCategory(c.id)"
          >
            {{ c.name }}
          </button>
        </div>
      </div>

      <label class="flex flex-col gap-1.5">
        <span class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Action</span>
        <select v-model="form.action" class="select select-bordered w-full text-[13.5px]">
          <option value="REVIEW">Review — wait in the Inbox</option>
          <option value="AUTO">Auto — book when the template is active</option>
          <option value="IGNORE">Ignore — never surface it</option>
        </select>
      </label>

      <label class="flex items-center gap-2 text-[13px] text-ink">
        <input v-model="form.enabled" type="checkbox" class="checkbox checkbox-sm" />
        Enabled
      </label>

      <div class="flex items-center justify-between border-t border-rule pt-3">
        <button v-if="editing" type="button" class="btn btn-sm btn-outline border-debit/30 text-debit hover:bg-debit/10" :disabled="saving" @click="remove">
          {{ confirmingDelete ? 'Confirm delete' : 'Delete' }}
        </button>
        <span v-else></span>
        <div class="flex items-center gap-2">
          <button type="button" class="btn btn-sm btn-ghost" @click="emit('close')">Cancel</button>
          <button type="submit" class="btn btn-sm btn-primary" :disabled="saving">{{ saving ? 'Saving…' : editing ? 'Save rule' : 'Create rule' }}</button>
        </div>
      </div>
    </form>
  </AppModal>
</template>
