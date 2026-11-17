<script setup lang="ts">
import { computed, ref } from 'vue';
import { mail } from '@/lib/api/client';
import {
  cleanedTemplate,
  emptyTemplate,
  TEMPLATE_FIELDS,
  templateProblem,
  type TemplateFieldKey,
  type TemplatePatterns
} from '@/lib/mailTemplate';

const props = defineProps<{ fromPattern: string }>();
const patterns = defineModel<TemplatePatterns | null>({ required: true });

const sample = ref('');
const sampleSubject = ref('');
const testing = ref(false);
const testError = ref('');
const result = ref<any | null>(null);

const enabled = computed(() => patterns.value !== null);
const canUse = computed(() => props.fromPattern.trim().length > 0);

function toggle() {
  patterns.value = patterns.value ? null : emptyTemplate();
  result.value = null;
  testError.value = '';
}

function addPattern(field: TemplateFieldKey) {
  if (!patterns.value) return;
  patterns.value = { ...patterns.value, [field]: [...patterns.value[field], ''] };
}

function removePattern(field: TemplateFieldKey, index: number) {
  if (!patterns.value) return;
  const list = patterns.value[field];
  if (list.length <= 1) return;
  patterns.value = { ...patterns.value, [field]: list.filter((_, i) => i !== index) };
  result.value = null;
}

function setPattern(field: TemplateFieldKey, index: number, value: string) {
  if (!patterns.value) return;
  const list = [...patterns.value[field]];
  list[index] = value;
  patterns.value = { ...patterns.value, [field]: list };
  result.value = null;
}

/** The validation row for a given pattern string, if one was returned. */
function checkFor(field: TemplateFieldKey, pattern: string) {
  if (!result.value) return null;
  const group = (result.value.fields || []).find((f: any) => f.field === field);
  if (!group) return null;
  return (group.patterns || []).find((p: any) => p.pattern === pattern.trim()) || null;
}

const problem = computed(() => (patterns.value ? templateProblem(patterns.value) : null));

async function test() {
  if (!patterns.value) return;
  testError.value = '';
  result.value = null;
  if (!sample.value.trim()) {
    testError.value = 'Paste a sample mail body to test.';
    return;
  }
  testing.value = true;
  try {
    result.value = await mail().validateTemplate({
      subject: sampleSubject.value,
      body: sample.value,
      patterns: cleanedTemplate(patterns.value)
    });
  } catch (e: any) {
    testError.value = e?.message || 'Could not test the template';
  }
  testing.value = false;
}
</script>

<template>
  <section class="flex flex-col gap-3 rounded-folio border border-rule p-3.5">
    <div class="flex items-start justify-between gap-3">
      <div class="flex flex-col gap-0.5">
        <span class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Extraction template</span>
        <span class="text-[11.5px] text-ink-mute">Manual › learned › AI. AI never overwrites this.</span>
      </div>
      <label class="flex shrink-0 items-center gap-2 text-[13px] text-ink">
        <input type="checkbox" class="checkbox checkbox-sm" :checked="enabled" :disabled="!canUse" @change="toggle" />
        Use template
      </label>
    </div>

    <p v-if="!canUse" class="text-[12px] text-ink-mute">
      Templates belong to a sender. Add a <span class="font-medium text-ink">From contains</span> condition to use one.
    </p>

    <template v-else-if="enabled && patterns">
      <p class="text-[11.5px] text-ink-mute">
        Go RE2 syntax. Name the value’s group (<code class="font-mono">(?P&lt;amount&gt;…)</code>) or leave one capture
        group. Patterns are tried top to bottom; the first that yields a value wins.
      </p>

      <div v-for="field in TEMPLATE_FIELDS" :key="field.key" class="flex flex-col gap-1.5">
        <div class="flex items-center gap-1.5">
          <span class="text-[12px] font-medium text-ink">{{ field.label }}</span>
          <span v-if="field.required" class="text-[11px] text-debit" aria-label="required">*</span>
        </div>

        <div v-for="(pattern, index) in patterns[field.key]" :key="index" class="flex items-center gap-1.5">
          <input
            :value="pattern"
            :aria-label="`${field.label} pattern ${index + 1}`"
            :placeholder="index === 0 ? 'e.g. Rs\\.?\\s*(?P<amount>[\\d,]+(?:\\.\\d{1,2})?)' : 'another shape for this field'"
            class="h-[34px] w-full rounded-control border border-rule bg-page-sunk px-2.5 font-mono text-[12px] text-ink placeholder:text-ink-faint focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20"
            spellcheck="false"
            @input="setPattern(field.key, index, ($event.target as HTMLInputElement).value)"
          />
          <span
            v-if="checkFor(field.key, pattern)?.error"
            role="status"
            class="stamp shrink-0 border-debit/40 text-debit"
            :title="checkFor(field.key, pattern).error"
          >invalid</span>
          <span
            v-else-if="checkFor(field.key, pattern)?.matched"
            role="status"
            class="stamp shrink-0 border-credit/40 text-credit"
          >{{ checkFor(field.key, pattern).value }}</span>
          <span v-else-if="checkFor(field.key, pattern)" role="status" class="stamp shrink-0 text-ink-mute">no match</span>
          <button
            type="button"
            class="flex h-8 w-8 shrink-0 items-center justify-center rounded-control text-ink-mute transition-colors hover:bg-page-sunk hover:text-debit disabled:opacity-40"
            :aria-label="`Remove ${field.label} pattern ${index + 1}`"
            :disabled="patterns[field.key].length <= 1"
            @click="removePattern(field.key, index)"
          >&times;</button>
        </div>

        <button
          type="button"
          class="self-start rounded-control px-1 text-[11.5px] text-ink-mute transition-colors hover:text-accent"
          @click="addPattern(field.key)"
        >+ Add {{ field.label.toLowerCase() }} pattern</button>
        <span v-if="field.hint" class="text-[11px] text-ink-mute">{{ field.hint }}</span>
      </div>

      <p v-if="problem" class="text-[12px] text-stamp">{{ problem }}</p>

      <div class="flex flex-col gap-2 border-t border-rule pt-3">
        <span class="text-[12px] font-medium text-ink">Test against a mail</span>
        <input
          v-model="sampleSubject"
          aria-label="Sample subject"
          placeholder="Subject (optional)"
          class="h-[34px] w-full rounded-control border border-rule bg-page-sunk px-2.5 text-[13px] text-ink placeholder:text-ink-faint focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20"
        />
        <textarea
          v-model="sample"
          aria-label="Sample mail body"
          rows="3"
          placeholder="Paste the body of a real mail from this sender"
          class="w-full rounded-control border border-rule bg-page-sunk px-2.5 py-2 font-mono text-[12px] text-ink placeholder:text-ink-faint focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20"
        />
        <div class="flex items-center gap-2">
          <button type="button" class="btn btn-sm btn-outline" :disabled="testing" @click="test">
            {{ testing ? 'Testing…' : 'Test template' }}
          </button>
          <span v-if="testError" role="alert" class="text-[12px] text-debit">{{ testError }}</span>
        </div>

        <div v-if="result" aria-live="polite" class="flex flex-col gap-1">
          <p v-if="result.reason && !result.complete" class="text-[12px] text-stamp">
            {{ result.complete ? '' : result.reason }}
          </p>
          <div v-if="result.amount || result.date" class="flex flex-wrap items-center gap-2 text-[12.5px] text-ink">
            <span v-if="result.amount" class="tabular-nums font-medium">{{ Number(result.amount).toLocaleString() }}</span>
            <span v-if="result.direction" class="stamp" :class="result.direction === 'DEBIT' ? 'text-debit border-debit/40' : 'text-credit border-credit/40'">{{ result.direction }}</span>
            <span v-if="result.account" class="stamp text-ink-mute">···{{ result.account }}</span>
            <span v-if="result.merchant" class="text-ink-soft">{{ result.merchant }}</span>
            <span v-if="result.date" class="text-ink-soft">{{ result.date }}</span>
          </div>
          <p v-else-if="result.fields?.length" class="text-[12px] text-ink-mute">No values extracted from this mail.</p>
        </div>
      </div>
    </template>
  </section>
</template>
