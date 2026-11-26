<script setup lang="ts">
export interface BandFact {
  label: string;
  value: string;
  /** Figures are mono by default; a wordy fact (an account type) opts out. */
  mono?: boolean;
  tone?: 'ink' | 'debit';
}

export interface BandBar {
  /** 0–100. */
  percent: number;
  color: string;
  /** The remainder of the track, for a composition bar. */
  restPercent?: number;
  restColor?: string;
  /** A marker for "where the calendar is", when the bar is a pace gauge. */
  tickPercent?: number;
}

// The money band: one figure leads, a row of facts explains it, one bar gives it
// scale. Accounts and Spending both open with this shape, so it lives here once.
withDefaults(
  defineProps<{
    label: string;
    value: string;
    valueTone?: 'ink' | 'debit';
    facts: BandFact[];
    bar?: BandBar | null;
    note?: string;
  }>(),
  { valueTone: 'ink', bar: null, note: '' }
);
</script>

<template>
  <section class="card rounded-card border border-rule bg-page px-4 pb-3 pt-4 md:px-5" aria-live="polite">
    <div class="flex flex-wrap items-end justify-between gap-x-8 gap-y-4">
      <div class="min-w-0">
        <p class="text-label font-medium uppercase tracking-[0.1em] text-ink-mute">{{ label }}</p>
        <p
          class="num mt-2 text-[30px] leading-none md:text-[38px]"
          :class="valueTone === 'debit' ? 'text-debit' : 'text-ink'"
        >
          {{ value }}
        </p>
        <slot name="under" />
      </div>

      <dl class="grid grid-cols-2 gap-x-6 gap-y-3 sm:flex sm:items-end sm:gap-0">
        <div
          v-for="fact in facts"
          :key="fact.label"
          class="min-w-0 sm:border-l sm:border-rule-soft sm:px-5 sm:first:border-l-0 sm:first:pl-0 sm:last:pr-0"
        >
          <dt class="text-label font-medium uppercase tracking-[0.1em] text-ink-mute">{{ fact.label }}</dt>
          <dd
            class="mt-2 truncate"
            :class="
              fact.mono === false
                ? 'text-[13px] text-ink-soft'
                : ['num text-[15px]', fact.tone === 'debit' ? 'text-debit' : 'text-ink']
            "
          >
            {{ fact.value }}
          </dd>
        </div>
      </dl>
    </div>

    <div v-if="bar" class="mt-4 flex flex-wrap items-center gap-3 border-t border-rule-soft pt-3">
      <div class="relative min-w-[120px] flex-1" role="img" :aria-label="note || `${Math.round(bar.percent)}%`">
        <div class="flex h-2 w-full overflow-hidden rounded-full bg-page-sunk">
          <span
            class="h-full"
            :style="{ width: `${Math.min(100, Math.max(0, bar.percent))}%`, background: bar.color }"
          ></span>
          <span
            v-if="bar.restPercent"
            class="h-full"
            :style="{ width: `${bar.restPercent}%`, background: bar.restColor || 'var(--rule)' }"
          ></span>
        </div>
        <span
          v-if="bar.tickPercent !== undefined"
          class="absolute -bottom-1 -top-1 w-[2px] rounded-full bg-ink-faint"
          :style="{ left: `${bar.tickPercent}%` }"
        ></span>
      </div>
      <p v-if="note" class="text-[11.5px] text-ink-mute">{{ note }}</p>
    </div>
  </section>
</template>
