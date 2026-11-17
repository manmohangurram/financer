<script setup lang="ts">
import { computed, ref } from 'vue';
import AppStamp from '@/components/ui/AppStamp.vue';
import { mail } from '@/lib/api/client';
import {
  cleanedTemplate,
  FIELD_VIEWS,
  type TemplateFieldView,
  type TemplatePatternKey,
  type TemplatePatterns
} from '@/lib/mailTemplate';

const patterns = defineModel<TemplatePatterns>({ required: true });

const selected = ref(FIELD_VIEWS[0].key);
const sampleSubject = ref('');
const sample = ref('');
const testing = ref(false);
const testError = ref('');
const result = ref<any | null>(null);

const field = computed<TemplateFieldView>(
  () => FIELD_VIEWS.find((f) => f.key === selected.value) || FIELD_VIEWS[0]
);

function list(key: TemplatePatternKey) {
  return patterns.value[key] || [];
}

function setPattern(key: TemplatePatternKey, index: number, value: string) {
  const next = [...list(key)];
  next[index] = value;
  patterns.value = { ...patterns.value, [key]: next };
  result.value = null;
}

function addPattern(key: TemplatePatternKey) {
  patterns.value = { ...patterns.value, [key]: [...list(key), ''] };
}

function removePattern(key: TemplatePatternKey, index: number) {
  if (list(key).length <= 1) return;
  patterns.value = { ...patterns.value, [key]: list(key).filter((_, i) => i !== index) };
  result.value = null;
}

function filled(view: TemplateFieldView) {
  return view.parts.some((part) => list(part.key).some((p) => p.trim().length > 0));
}

/** Pattern results by field key for the current test run. */
const checks = computed(() => {
  const map: Record<string, Record<string, any>> = {};
  for (const group of result.value?.fields || []) {
    map[group.field] = {};
    for (const check of group.patterns || []) map[group.field][check.pattern] = check;
  }
  return map;
});

function checkFor(key: TemplatePatternKey, pattern: string) {
  return checks.value[key]?.[pattern.trim()] || null;
}

const patternCount = computed(() => field.value.parts.reduce((n, p) => n + list(p.key).length, 0));

const missing = computed(() => {
  const clean = cleanedTemplate(patterns.value);
  return {
    amount: clean.amount.length === 0,
    date: clean.date.length === 0,
    direction: clean.debit.length === 0 && clean.credit.length === 0
  };
});

async function test() {
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
  <div class="flex h-full min-h-0 gap-6">
    <nav class="flex w-[212px] shrink-0 flex-col gap-0.5 overflow-y-auto border-r border-rule pr-4" aria-label="Template fields">
      <p class="mb-2 text-[10.5px] uppercase tracking-[0.07em] text-ink-faint">Fields</p>
      <button
        v-for="view in FIELD_VIEWS"
        :key="view.key"
        type="button"
        class="flex items-center gap-2.5 rounded-control px-2.5 py-2 text-left text-[13px] transition-colors"
        :class="view.key === selected ? 'bg-accent-wash font-semibold text-ink' : 'text-ink-soft hover:bg-page-sunk'"
        :aria-current="view.key === selected ? 'true' : undefined"
        @click="selected = view.key"
      >
        <span
          class="h-[7px] w-[7px] shrink-0 rounded-full"
          :class="filled(view) ? 'bg-credit' : 'border border-ink-faint'"
          aria-hidden="true"
        />
        <span class="truncate">{{ view.label }}</span>
        <span v-if="view.required" class="ml-auto text-[10px] text-debit" aria-label="required">*</span>
      </button>

      <p class="mt-4 mb-2 text-[10.5px] uppercase tracking-[0.07em] text-ink-faint">Test</p>
      <div class="flex items-center gap-2.5 px-2.5 py-2 text-[12.5px] text-ink-soft">
        <span class="h-[7px] w-[7px] shrink-0 rounded-full" :class="result ? 'bg-credit' : 'border border-ink-faint'" aria-hidden="true" />
        <span>{{ result ? 'Tested' : 'Not run yet' }}</span>
        <AppStamp v-if="result?.complete" tone="credit" class="ml-auto">complete</AppStamp>
      </div>
    </nav>

    <div class="flex min-h-0 flex-1 flex-col">
      <div class="min-h-0 flex-1 overflow-y-auto pb-3 pr-2">
        <div class="flex items-center justify-between gap-3">
          <h4 class="m-0 text-[15px] font-semibold text-ink">{{ field.label }}</h4>
          <AppStamp tone="accent">manual</AppStamp>
        </div>

        <div v-for="(part, partIndex) in field.parts" :key="part.key" :class="partIndex > 0 ? 'mt-5 border-t border-rule-soft pt-4' : ''">
          <p class="mt-1.5 mb-0 text-[12px] leading-relaxed text-ink-mute">
            <template v-if="field.parts.length === 1">{{ field.hint }}</template>
            <template v-else>
              <span class="block text-[12px] font-semibold text-ink-soft">{{ part.label }}</span>
              {{ part.hint }}
            </template>
          </p>

          <div class="mt-3 flex flex-col gap-2">
            <div v-for="(pattern, index) in list(part.key)" :key="index" class="flex items-center gap-2">
              <input
                :value="pattern"
                :aria-label="`${part.label || field.label} pattern ${index + 1}`"
                :placeholder="index === 0 ? 'Rs\\.?\\s*(?P<amount>[\\d,]+(?:\\.\\d{1,2})?)' : 'another shape for this field'"
                class="h-[42px] w-full rounded-control border border-rule bg-page-sunk px-3 font-mono text-[12.5px] text-ink placeholder:text-ink-faint focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20"
                spellcheck="false"
                @input="setPattern(part.key, index, ($event.target as HTMLInputElement).value)"
              />
              <AppStamp v-if="checkFor(part.key, pattern)?.error" tone="debit" :title="checkFor(part.key, pattern).error">invalid</AppStamp>
              <AppStamp v-else-if="checkFor(part.key, pattern)?.matched" tone="credit">{{ checkFor(part.key, pattern).value }}</AppStamp>
              <AppStamp v-else-if="checkFor(part.key, pattern)" tone="neutral">no match</AppStamp>
              <button
                type="button"
                class="flex h-8 w-8 shrink-0 items-center justify-center rounded-control text-ink-mute transition-colors hover:bg-page-sunk hover:text-debit disabled:opacity-40"
                :aria-label="`Remove ${part.label || field.label} pattern ${index + 1}`"
                :disabled="list(part.key).length <= 1"
                @click="removePattern(part.key, index)"
              >&times;</button>
            </div>
            <button type="button" class="self-start text-[12px] text-ink-mute transition-colors hover:text-accent" @click="addPattern(part.key)">
              + Add another {{ (part.label || field.label).toLowerCase() }} pattern
            </button>
          </div>
        </div>

        <p class="mt-4 border-l-2 border-accent pl-3 text-[12px] leading-relaxed text-ink-mute">
          Patterns are tried top to bottom. The first one that produces a value wins — later ones cover other mail shapes.
        </p>

        <p v-if="missing.amount || missing.date || missing.direction" class="mt-3 text-[12px] text-stamp">
          Still needed:
          <span v-if="missing.amount">amount</span>
          <span v-if="missing.amount && (missing.date || missing.direction)">, </span>
          <span v-if="missing.date">date</span>
          <span v-if="missing.date && missing.direction">, </span>
          <span v-if="missing.direction">debit or credit</span>.
        </p>
      </div>

      <div class="mt-3 shrink-0 border-t border-rule pt-3.5">
        <div class="flex items-center justify-between gap-3">
          <label class="text-[11px] font-medium uppercase tracking-[0.06em] text-ink-mute" for="template-sample">Test against a mail</label>
          <span v-if="patternCount > 4" class="text-[11px] text-ink-faint">{{ patternCount }} patterns · scroll the list</span>
        </div>
        <div class="mt-2 flex flex-col gap-2">
          <input
            v-model="sampleSubject"
            aria-label="Sample subject"
            placeholder="Subject (optional)"
            class="h-[34px] w-full rounded-control border border-rule bg-page-sunk px-2.5 text-[13px] text-ink placeholder:text-ink-faint focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20"
          />
          <textarea
            id="template-sample"
            v-model="sample"
            rows="2"
            placeholder="Paste the body of a real mail from this sender"
            class="w-full resize-none rounded-control border border-rule bg-page-sunk px-2.5 py-2 font-mono text-[12px] text-ink placeholder:text-ink-faint focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20"
          />
        </div>
        <div class="mt-2 flex flex-wrap items-center gap-2" aria-live="polite">
          <button type="button" class="btn btn-sm btn-outline" :disabled="testing" @click="test">
            {{ testing ? 'Testing…' : 'Test template' }}
          </button>
          <span v-if="testError" role="alert" class="text-[12px] text-debit">{{ testError }}</span>
          <template v-else-if="result">
            <AppStamp v-if="result.amount" tone="credit">{{ Number(result.amount).toLocaleString() }}</AppStamp>
            <AppStamp v-if="result.direction" :tone="result.direction === 'DEBIT' ? 'debit' : 'credit'">{{ result.direction }}</AppStamp>
            <AppStamp v-if="result.account">···{{ result.account }}</AppStamp>
            <AppStamp v-if="result.merchant && result.merchant !== 'Unknown'">{{ result.merchant }}</AppStamp>
            <AppStamp v-if="result.date">{{ result.date }}</AppStamp>
            <AppStamp v-if="result.complete" tone="credit">complete</AppStamp>
            <span v-else-if="result.reason" class="text-[12px] text-stamp">{{ result.reason }}</span>
          </template>
          <span v-else class="text-[11.5px] text-ink-mute">Not tested yet</span>
        </div>
      </div>
    </div>
  </div>
</template>
