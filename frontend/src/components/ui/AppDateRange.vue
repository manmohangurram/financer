<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { CalendarDays, ChevronLeft, ChevronRight } from '@lucide/vue';

// The period picker: the dates already on screen are the control. Click them,
// pick a start day and an end day, apply. Presets live beside it in the header.
const props = withDefaults(
  defineProps<{
    from: string;
    to: string;
    /** What the trigger reads, e.g. `20–26 Nov 2026`. */
    label: string;
    /** True while the page is on a hand-picked range. */
    active?: boolean;
  }>(),
  { active: false }
);
const emit = defineEmits<{ (e: 'apply', range: { from: string; to: string }): void }>();

const open = ref(false);
const root = ref<HTMLElement | null>(null);
const viewYear = ref(new Date().getFullYear());
const viewMonth = ref(new Date().getMonth());
// The draft range: survives month changes, thrown away on Cancel/Esc.
const from = ref('');
const to = ref('');
// The one day the grid's tab stop sits on, so Tab enters the calendar once.
const focusIso = ref('');

const WEEKDAYS = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa'];

const iso = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const monthPrefix = () => `${viewYear.value}-${String(viewMonth.value + 1).padStart(2, '0')}`;

/** `20 Nov` — the footer reads a date, not a machine stamp. */
function short(value: string): string {
  if (!value) return '';
  const d = new Date(`${value}T00:00:00`);
  if (isNaN(d.getTime())) return value;
  return `${d.getDate()} ${d.toLocaleDateString(undefined, { month: 'short' })}`;
}

const weeks = computed<Array<(Date | null)[]>>(() => {
  const first = new Date(viewYear.value, viewMonth.value, 1);
  const daysInMonth = new Date(viewYear.value, viewMonth.value + 1, 0).getDate();
  const cells: (Date | null)[] = [];
  for (let i = 0; i < first.getDay(); i++) cells.push(null);
  for (let d = 1; d <= daysInMonth; d++) cells.push(new Date(viewYear.value, viewMonth.value, d));
  while (cells.length % 7 !== 0) cells.push(null);
  const out: Array<(Date | null)[]> = [];
  for (let i = 0; i < cells.length; i += 7) out.push(cells.slice(i, i + 7));
  return out;
});

const monthLabel = computed(() =>
  new Date(viewYear.value, viewMonth.value, 1).toLocaleDateString(undefined, { month: 'long', year: 'numeric' })
);
const low = computed(() => (from.value && to.value && from.value > to.value ? to.value : from.value));
const high = computed(() => (from.value && to.value && from.value > to.value ? from.value : to.value));
const canApply = computed(() => from.value !== '' && to.value !== '');
const pickedLabel = computed(() => {
  if (!from.value) return 'Pick a start day';
  if (!to.value) return `${short(from.value)} → pick an end day`;
  return `${short(low.value)} → ${short(high.value)}`;
});

const cellFor = (value: string) => root.value?.querySelector<HTMLElement>(`[data-day="${value}"]`) ?? null;

async function toggle() {
  if (open.value) {
    open.value = false;
    return;
  }
  open.value = true;
  from.value = props.from;
  to.value = props.to;
  const anchor = props.to || props.from;
  const base = anchor ? new Date(`${anchor}T00:00:00`) : new Date();
  viewYear.value = base.getFullYear();
  viewMonth.value = base.getMonth();
  // Start the keyboard where the selection is, else on the first of the month.
  const preferred = props.to || props.from || iso(new Date());
  focusIso.value = preferred.startsWith(monthPrefix()) ? preferred : `${monthPrefix()}-01`;
  await nextTick();
  cellFor(focusIso.value)?.focus();
}

function step(months: number) {
  const d = new Date(viewYear.value, viewMonth.value + months, 1);
  viewYear.value = d.getFullYear();
  viewMonth.value = d.getMonth();
}

/** Arrow keys move the focus day, and follow it into the next month. */
async function moveFocus(days: number) {
  const base = focusIso.value || `${monthPrefix()}-01`;
  const current = new Date(`${base}T00:00:00`);
  if (isNaN(current.getTime())) return;
  const target = new Date(current.getFullYear(), current.getMonth(), current.getDate() + days);
  if (target.getMonth() !== viewMonth.value || target.getFullYear() !== viewYear.value) {
    viewYear.value = target.getFullYear();
    viewMonth.value = target.getMonth();
  }
  focusIso.value = iso(target);
  await nextTick();
  cellFor(focusIso.value)?.focus();
}

const ARROW_STEPS: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 };
function onGridKey(event: KeyboardEvent) {
  const stepDays = ARROW_STEPS[event.key];
  if (stepDays === undefined) return;
  event.preventDefault();
  void moveFocus(stepDays);
}

/** First click sets the start, second sets the end; a third starts over. */
function pick(day: Date) {
  const value = iso(day);
  if (!from.value || to.value) {
    from.value = value;
    to.value = '';
    return;
  }
  to.value = value;
}

function apply() {
  if (!canApply.value) return;
  emit('apply', { from: low.value, to: high.value });
  open.value = false;
}

const isEdge = (day: Date) => iso(day) === low.value || iso(day) === high.value;
const inRange = (day: Date) => {
  const value = iso(day);
  return !!low.value && !!high.value && value > low.value && value < high.value;
};
const isToday = (day: Date) => iso(day) === iso(new Date());

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false;
}
watch(open, (isOpen) => {
  if (isOpen) document.addEventListener('keydown', onKey);
  else document.removeEventListener('keydown', onKey);
});
onBeforeUnmount(() => document.removeEventListener('keydown', onKey));
</script>

<template>
  <div class="relative">
    <button
      type="button"
      class="btn btn-ghost h-8 min-h-0 gap-1.5 rounded-control px-2 text-[12px] font-normal"
      :class="active ? 'bg-accent-wash text-accent' : 'text-ink-soft hover:bg-page-sunk hover:text-ink'"
      aria-haspopup="dialog"
      :aria-expanded="open"
      aria-label="Pick a date range"
      @click="toggle"
    >
      <CalendarDays class="h-[14px] w-[14px]" stroke-width="1.7" aria-hidden="true" />
      <span class="num">{{ label }}</span>
    </button>

    <template v-if="open">
      <div class="fixed inset-0 z-40" aria-hidden="true" @click="open = false"></div>
      <div
        ref="root"
        role="dialog"
        :aria-label="`Pick a date range, ${monthLabel}`"
        class="absolute right-0 top-full z-50 mt-2 w-[268px] rounded-card border border-rule bg-page p-3 shadow-lift"
      >
        <div class="mb-2 flex items-center justify-between">
          <button
            type="button"
            class="btn btn-ghost btn-sm h-7 min-h-0 w-7 p-0 text-ink-mute"
            aria-label="Previous month"
            @click="step(-1)"
          >
            <ChevronLeft class="h-4 w-4" stroke-width="1.8" />
          </button>
          <span class="text-[12.5px] font-medium text-ink" aria-live="polite">{{ monthLabel }}</span>
          <button
            type="button"
            class="btn btn-ghost btn-sm h-7 min-h-0 w-7 p-0 text-ink-mute"
            aria-label="Next month"
            @click="step(1)"
          >
            <ChevronRight class="h-4 w-4" stroke-width="1.8" />
          </button>
        </div>

        <div class="grid grid-cols-7 gap-0.5 pb-1 text-center">
          <span v-for="w in WEEKDAYS" :key="w" class="text-[10px] uppercase tracking-[0.06em] text-ink-faint">
            {{ w }}
          </span>
        </div>

        <div class="flex flex-col gap-0.5" role="grid" :aria-label="monthLabel" @keydown="onGridKey">
          <div v-for="(week, i) in weeks" :key="i" class="grid grid-cols-7 gap-0.5" role="row">
            <button
              v-for="(day, j) in week"
              :key="j"
              type="button"
              role="gridcell"
              :disabled="!day"
              :data-day="day ? iso(day) : undefined"
              :tabindex="day && iso(day) === focusIso ? 0 : -1"
              class="btn btn-ghost h-8 min-h-0 w-8 p-0 text-[12px] font-normal"
              :class="
                !day
                  ? 'invisible'
                  : isEdge(day)
                    ? 'bg-accent text-on-accent hover:bg-accent'
                    : inRange(day)
                      ? 'bg-accent-wash text-ink hover:bg-accent-wash'
                      : isToday(day)
                        ? 'text-accent hover:bg-page-sunk'
                        : 'text-ink-soft hover:bg-page-sunk hover:text-ink'
              "
              :aria-selected="day ? isEdge(day) : undefined"
              :aria-label="
                day ? day.toLocaleDateString(undefined, { day: 'numeric', month: 'long', year: 'numeric' }) : undefined
              "
              @focus="day && (focusIso = iso(day))"
              @click="day && pick(day)"
            >
              {{ day?.getDate() }}
            </button>
          </div>
        </div>

        <div class="mt-3 flex items-center justify-between gap-2 border-t border-rule-soft pt-2.5">
          <span class="num text-[11px] text-ink-mute">{{ pickedLabel }}</span>
          <div class="flex items-center gap-1">
            <button type="button" class="btn btn-ghost btn-sm h-7 min-h-0 px-2 text-[12px]" @click="open = false">
              Cancel
            </button>
            <button
              type="button"
              class="btn btn-sm h-7 min-h-0 px-2.5 text-[12px]"
              :class="canApply ? 'btn-primary' : 'btn-disabled'"
              :disabled="!canApply"
              @click="apply"
            >
              Apply
            </button>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
