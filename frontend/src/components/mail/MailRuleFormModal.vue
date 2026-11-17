<script setup lang="ts">
import { computed, ref } from 'vue';
import AppModal from '@/components/AppModal.vue';
import AppStamp from '@/components/ui/AppStamp.vue';
import MailTemplateEditor from '@/components/mail/MailTemplateEditor.vue';
import { mail } from '@/lib/api/client';
import { cleanedTemplate, emptyTemplate, templateProblem, type TemplatePatterns } from '@/lib/mailTemplate';

const props = defineProps<{
  rule?: any | null;
  accounts?: any[];
  categories?: any[];
}>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>();

const editing = !!props.rule;
const steps = ['Match', 'Template', 'Booking'];
const step = ref(0);
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
const tested = ref(false);
const saving = ref(false);
const confirmingDelete = ref(false);
const error = ref('');

function toggleCategory(id: string) {
  form.value.categoryIds = form.value.categoryIds.includes(id)
    ? form.value.categoryIds.filter((x) => x !== id)
    : [...form.value.categoryIds, id];
}

function goTo(index: number) {
  if (index <= step.value) step.value = index;
}

function next() {
  error.value = '';
  if (step.value === 0) {
    if (!form.value.fromPattern.trim() && !form.value.subjectContains.trim() && !form.value.bodyContains.trim()) {
      error.value = 'Add at least one condition: sender, subject or body.';
      return;
    }
    // A template belongs to a sender; dropping the sender drops the template.
    if (!form.value.fromPattern.trim()) form.value.template = null;
    // Entering the template step starts one, so “Skip template” is a real choice.
    if (!form.value.template && form.value.fromPattern.trim()) form.value.template = emptyTemplate();
    step.value = 1;
    return;
  }
  if (step.value === 1) {
    if (form.value.template) {
      const problem = templateProblem(form.value.template);
      if (problem) {
        error.value = problem;
        return;
      }
    }
    step.value = 2;
  }
}

function skipTemplate() {
  form.value.template = null;
  tested.value = false;
  error.value = '';
  step.value = 2;
}

function back() {
  error.value = '';
  if (step.value > 0) step.value -= 1;
}

const summary = computed(() => {
  const clean = form.value.template ? cleanedTemplate(form.value.template) : null;
  const fields = clean
    ? Object.values(clean).filter((list) => list.length > 0).length
    : 0;
  return { fields, hasTemplate: !!clean };
});

async function submit() {
  error.value = '';
  if (form.value.template) {
    const problem = templateProblem(form.value.template);
    if (problem) {
      error.value = problem;
      step.value = 1;
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
  <AppModal :title="editing ? 'Edit mail rule' : 'New mail rule'" roomy steady @close="emit('close')">
    <div class="flex h-full min-h-0 flex-col px-[26px] pt-[18px] pb-[14px]">
      <ol class="flex shrink-0 items-center border-b border-rule pb-3.5" aria-label="Rule steps">
        <li
          v-for="(label, index) in steps"
          :key="label"
          class="flex items-center"
          :class="index < steps.length - 1 ? 'flex-1' : ''"
        >
          <button
            type="button"
            class="flex shrink-0 items-center gap-2 rounded-control px-1 py-0.5 text-[12.5px] transition-colors"
            :class="index === step ? 'font-semibold text-ink' : index < step ? 'text-ink-soft hover:text-ink' : 'text-ink-mute'"
            :disabled="index > step"
            :aria-current="index === step ? 'step' : undefined"
            @click="goTo(index)"
          >
            <span
              class="grid h-[22px] w-[22px] place-items-center rounded-full border text-[11px]"
              :class="index === step ? 'border-accent bg-accent text-page' : index < step ? 'border-accent text-accent' : 'border-rule-strong'"
            >{{ index < step ? '✓' : index + 1 }}</span>
            {{ label }}
          </button>
          <span v-if="index < steps.length - 1" class="mx-2.5 h-px flex-1 bg-rule" aria-hidden="true" />
        </li>
      </ol>

      <div class="flex min-h-0 flex-1 flex-col pt-4" :class="step === 0 ? 'overflow-y-auto' : 'overflow-hidden'">
        <p v-if="error" role="alert" class="mb-3 shrink-0 rounded-control border border-debit/30 bg-debit/5 px-3 py-2 text-[12.5px] text-debit">{{ error }}</p>

        <!-- Step 1: match -->
        <div v-if="step === 0" class="flex flex-col gap-5">
          <p class="m-0 text-[13px] leading-[1.5] text-ink-mute">
            Only mail that matches this rule is stored. Leave a field empty to ignore it — matching is case-insensitive.
          </p>
          <div class="flex flex-col gap-1.5">
            <label class="text-[11px] font-semibold uppercase tracking-[0.06em] text-ink-mute" for="rule-from">From contains</label>
            <input id="rule-from" v-model="form.fromPattern" placeholder="alerts@hdfcbank.net" class="h-10 w-full rounded-[4px] border border-rule bg-page-sunk px-3 text-[14px] text-ink placeholder:text-ink-faint transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20" />
            <span class="mt-1.5 text-[11.5px] leading-[1.5] text-ink-mute">
              A substring of the sender address. Use <code class="font-mono">hdfcbank</code> to cover more than one address.
            </span>
          </div>
          <div class="flex flex-col gap-1.5 border-t border-rule-soft pt-[18px]">
            <label class="text-[11px] font-semibold uppercase tracking-[0.06em] text-ink-mute" for="rule-subject">Subject contains</label>
            <input id="rule-subject" v-model="form.subjectContains" placeholder="Debit Alert" class="h-10 w-full rounded-[4px] border border-rule bg-page-sunk px-3 text-[14px] text-ink placeholder:text-ink-faint transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20" />
            <span class="mt-1.5 text-[11.5px] leading-[1.5] text-ink-mute">Optional.</span>
          </div>
          <div class="flex flex-col gap-1.5 border-t border-rule-soft pt-[18px]">
            <label class="text-[11px] font-semibold uppercase tracking-[0.06em] text-ink-mute" for="rule-body">Body contains</label>
            <input id="rule-body" v-model="form.bodyContains" placeholder="debited" class="h-10 w-full rounded-[4px] border border-rule bg-page-sunk px-3 text-[14px] text-ink placeholder:text-ink-faint transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20" />
            <span class="mt-1.5 text-[11.5px] leading-[1.5] text-ink-mute">Optional. Checked against the plain-text body.</span>
          </div>
        </div>

        <!-- Step 2: template -->
        <div v-else-if="step === 1 && form.template" class="min-h-0 flex-1">
          <MailTemplateEditor v-model="form.template" @tested="tested = true" />
        </div>

        <!-- Step 3: booking -->
        <div v-else class="flex min-h-0 flex-1 flex-col gap-6">
          <div class="flex flex-wrap items-center gap-2.5 rounded-control border border-rule bg-page-sunk px-3.5 py-2.5 text-[12.5px]">
            <span class="font-mono text-ink-soft">{{ form.fromPattern || '—' }}</span>
            <span class="text-ink-faint" aria-hidden="true">·</span>
            <span class="text-ink-mute">Template</span>
            <AppStamp v-if="summary.hasTemplate" tone="accent">{{ summary.fields }} fields</AppStamp>
            <AppStamp v-else>none</AppStamp>
            <span class="text-ink-faint" aria-hidden="true">·</span>
            <AppStamp v-if="tested" tone="credit">tested</AppStamp>
            <AppStamp v-else>not tested</AppStamp>
          </div>

          <div class="grid gap-8 sm:grid-cols-2">
            <section class="flex flex-col">
              <h4 class="m-0 text-[11px] font-medium uppercase tracking-[0.07em] text-ink-mute">Where it goes</h4>
              <label class="mt-4 flex flex-col gap-1.5">
                <span class="text-[11px] font-medium uppercase tracking-[0.06em] text-ink-mute">Book to account</span>
                <select v-model="form.accountId" class="select select-bordered h-10 w-full text-[13.5px]">
                  <option value="">No fixed account</option>
                  <option v-for="a in accounts || []" :key="a.id" :value="a.id">{{ a.nickname || a.bankName }}</option>
                </select>
              </label>
              <span class="mt-1.5 text-[11.5px] text-ink-mute">Used when the mail has no ending digits.</span>

              <div class="mt-5 flex flex-col gap-1.5">
                <span class="text-[11px] font-medium uppercase tracking-[0.06em] text-ink-mute">Categories</span>
                <div v-if="(categories || []).length" class="flex flex-wrap gap-1.5">
                  <button
                    v-for="c in categories"
                    :key="c.id"
                    type="button"
                    class="btn btn-xs"
                    :class="form.categoryIds.includes(c.id) ? 'btn-primary' : 'btn-outline'"
                    :aria-pressed="form.categoryIds.includes(c.id)"
                    @click="toggleCategory(c.id)"
                  >{{ c.name }}</button>
                </div>
                <span v-else class="text-[11.5px] text-ink-mute">No categories yet — add one under Categories.</span>
              </div>
            </section>

            <section class="flex flex-col">
              <h4 class="m-0 text-[11px] font-medium uppercase tracking-[0.07em] text-ink-mute">What happens</h4>
              <div role="radiogroup" aria-label="Rule action" class="mt-4 flex flex-col gap-2">
                <label
                  v-for="option in [
                    { value: 'REVIEW', title: 'Review', text: 'Wait in the Inbox for approval. Safest.' },
                    { value: 'AUTO', title: 'Auto', text: 'Book immediately when the template extracts cleanly.' },
                    { value: 'IGNORE', title: 'Ignore', text: 'Never store or surface this mail.' }
                  ]"
                  :key="option.value"
                  class="flex cursor-pointer items-start gap-3 rounded-control border px-3.5 py-3.5 transition-colors"
                  :class="form.action === option.value ? 'border-accent/50 bg-accent-wash' : 'border-rule hover:bg-page-sunk/60'"
                >
                  <input v-model="form.action" type="radio" :value="option.value" class="peer sr-only" />
                  <span
                    class="mt-0.5 grid h-4 w-4 shrink-0 place-items-center rounded-full border transition-colors"
                    :class="form.action === option.value ? 'border-accent' : 'border-rule-strong'"
                    aria-hidden="true"
                  >
                    <span v-if="form.action === option.value" class="h-2 w-2 rounded-full bg-accent" />
                  </span>
                  <span class="flex flex-col">
                    <span class="text-[13px] font-medium text-ink">{{ option.title }}</span>
                    <span class="text-[11.5px] text-ink-mute">{{ option.text }}</span>
                  </span>
                </label>
              </div>
            </section>
          </div>

          <div class="mt-auto flex items-center justify-between gap-4 border-t border-rule pt-5">
            <div class="flex items-center gap-3">
              <button
                type="button"
                role="switch"
                :aria-checked="form.enabled"
                aria-label="Rule is enabled"
                class="relative h-5 w-9 shrink-0 rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
                :class="form.enabled ? 'bg-accent' : 'bg-rule-strong'"
                @click="form.enabled = !form.enabled"
              >
                <span
                  class="absolute top-0.5 h-4 w-4 rounded-full bg-page transition-all"
                  :class="form.enabled ? 'left-[18px]' : 'left-0.5'"
                  aria-hidden="true"
                />
              </button>
              <span class="text-[13px] text-ink">Rule is enabled</span>
            </div>
            <span class="text-[11.5px] text-ink-mute">You can change all of this later.</span>
          </div>
        </div>
      </div>

      <div class="mt-3 flex shrink-0 items-center justify-between gap-2 border-t border-rule pt-[14px]">
        <div class="flex items-center gap-2">
          <button v-if="step === 0" type="button" class="btn btn-sm btn-ghost" @click="emit('close')">Cancel</button>
          <button v-else type="button" class="btn btn-sm btn-ghost" @click="back">Back</button>
          <button
            v-if="editing && step === 2"
            type="button"
            class="btn btn-sm btn-outline border-debit/30 text-debit hover:bg-debit/10"
            :disabled="saving"
            @click="remove"
          >{{ confirmingDelete ? 'Confirm delete' : 'Delete' }}</button>
        </div>
        <div class="flex items-center gap-2">
          <button v-if="step === 1" type="button" class="btn btn-sm btn-ghost" @click="skipTemplate">Skip template</button>
          <button v-if="step < 2" type="button" class="btn btn-sm btn-primary" @click="next">Continue</button>
          <button v-else type="button" class="btn btn-sm btn-primary" :disabled="saving" @click="submit">
            {{ saving ? 'Saving…' : editing ? 'Save rule' : 'Create rule' }}
          </button>
        </div>
      </div>
    </div>
  </AppModal>
</template>
