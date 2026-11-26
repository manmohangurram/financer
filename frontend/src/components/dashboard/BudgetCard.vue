<script setup lang="ts">
import { computed } from 'vue';
import type { DashboardMonth } from '@/lib/api/client';
import { formatAmount } from '@/lib/utils/format';
import { budgetColor } from '@/lib/utils/budget';
import AppStamp from '@/components/ui/AppStamp.vue';

const props = defineProps<{ month: DashboardMonth }>();

const ring = 2 * Math.PI * 52;
const money = (value: number) => formatAmount(Math.round(value));

const budget = computed(() => props.month.budget);
const over = computed(() => budget.value.left < 0);
const used = computed(() => Math.min(100, Math.max(0, budget.value.used)));
const dash = computed(() => (used.value / 100) * ring);
const usedLabel = computed(() => `${budget.value.used.toFixed(1)}%`);
const leftLabel = computed(() => money(Math.abs(budget.value.left)));
const endLabel = computed(() => {
  const d = new Date(`${props.month.to}T00:00:00`);
  return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
});
</script>

<template>
  <section
    class="card flex flex-col overflow-hidden rounded-card border border-rule bg-page p-4 md:p-5"
    aria-labelledby="budget-title"
  >
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <p id="budget-title" class="text-label font-medium uppercase tracking-[0.1em] text-ink-mute">Budget</p>
        <p class="mt-1.5 truncate text-[11.5px] text-ink-mute">Monthly limits · {{ month.label }}</p>
      </div>
      <AppStamp v-if="budget.overCount > 0" tone="pending">{{ budget.overCount }} over</AppStamp>
    </div>

    <template v-if="budget.hasBudgets">
      <div class="mt-4 flex items-center gap-4 md:gap-5">
        <div class="relative h-[116px] w-[116px] shrink-0">
          <svg viewBox="0 0 120 120" class="h-full w-full" aria-hidden="true">
            <circle cx="60" cy="60" r="52" fill="none" stroke="var(--page-sunk)" stroke-width="9" />
            <circle
              cx="60"
              cy="60"
              r="52"
              fill="none"
              :stroke="budgetColor(budget.used)"
              stroke-width="9"
              stroke-linecap="round"
              :stroke-dasharray="`${dash} ${ring}`"
              transform="rotate(-90 60 60)"
            />
          </svg>
          <div
            class="absolute inset-0 grid place-content-center text-center"
            role="progressbar"
            :aria-valuenow="budget.used"
            aria-valuemin="0"
            aria-valuemax="100"
            aria-label="Budget used"
          >
            <span class="num text-[19px] leading-none text-ink">{{ usedLabel }}</span>
            <small class="mt-1 text-[9px] uppercase tracking-[0.1em] text-ink-mute">used</small>
          </div>
        </div>

        <div class="min-w-0 flex-1">
          <p class="text-label font-medium uppercase tracking-[0.1em] text-ink-mute">
            {{ over ? 'Over budget' : 'Left to spend' }}
          </p>
          <p
            class="num mt-1.5 flex flex-wrap items-baseline gap-x-2 text-[27px] leading-none"
            :class="over ? 'text-debit' : 'text-ink'"
          >
            {{ over ? `−${leftLabel}` : leftLabel }}
            <span class="font-ui text-[11.5px] font-normal tracking-[0] text-ink-mute">
              of {{ money(budget.planned) }} planned
            </span>
          </p>
          <dl class="mt-3 grid grid-cols-2 gap-y-1.5 text-[12px]">
            <dt class="text-ink-mute">Spent</dt>
            <dd class="num m-0 text-right text-ink-soft">{{ money(budget.spent) }}</dd>
            <dt class="text-ink-mute">Planned</dt>
            <dd class="num m-0 text-right text-ink-soft">{{ money(budget.planned) }}</dd>
          </dl>
        </div>
      </div>

      <div
        class="mt-4 flex items-center justify-between gap-3 border-t border-rule-soft pt-3 text-[11.5px] text-ink-mute"
      >
        <span>{{ month.daysLeft }} days left · ends {{ endLabel }}</span>
        <router-link to="/accounts/categories" class="link link-hover text-[11.5px] font-medium text-accent"
          >Adjust →</router-link
        >
      </div>
    </template>

    <div v-else class="mt-4 flex flex-1 flex-col justify-center gap-2">
      <p class="text-[13px] text-ink">No monthly budget yet.</p>
      <router-link to="/accounts/categories" class="link link-hover text-[12px] font-medium text-accent">
        Set monthly limits →
      </router-link>
    </div>
  </section>
</template>
