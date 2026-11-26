<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { ChevronLeft, ChevronRight } from '@lucide/vue';
import AppMenu from '@/components/ui/AppMenu.vue';

const model = defineModel<string | undefined>({ required: true });
const props = withDefaults(defineProps<{ placeholder?: string; class?: string }>(), { placeholder: 'Pick a date' });

const open = ref(false);
const root = ref<HTMLElement | null>(null);
const viewYear = ref(new Date().getFullYear());
const viewMonth = ref(new Date().getMonth());
// The grid's single tab stop, so Tab enters the calendar once, not 31 times.
const focusIso = ref('');

const buttonCls = computed(() =>
  ['input w-full bg-page text-left', model.value ? 'text-ink' : 'text-ink-soft', props.class].join(' ')
);

const iso = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const monthPrefix = () => `${viewYear.value}-${String(viewMonth.value + 1).padStart(2, '0')}`;

/** `20 Nov 2026` — what the field and a screen reader both read. */
function readable(d: Date): string {
  return d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
}

const selectedDate = computed<Date | null>(() => {
  if (!model.value) return null;
  const [y, m, d] = model.value.split('-').map(Number);
  const dt = new Date(y, m - 1, d);
  return isNaN(dt.getTime()) ? null : dt;
});

const weeks = computed<Array<(Date | null)[]>>(() => {
  const first = new Date(viewYear.value, viewMonth.value, 1);
  const startWeekday = first.getDay();
  const daysInMonth = new Date(viewYear.value, viewMonth.value + 1, 0).getDate();
  const cells: (Date | null)[] = [];
  for (let i = 0; i < startWeekday; i++) cells.push(null);
  for (let d = 1; d <= daysInMonth; d++) cells.push(new Date(viewYear.value, viewMonth.value, d));
  while (cells.length % 7 !== 0) cells.push(null);
  const out: Array<(Date | null)[]> = [];
  for (let i = 0; i < cells.length; i += 7) out.push(cells.slice(i, i + 7));
  return out;
});

const monthOptions = computed(() =>
  Array.from({ length: 12 }, (_, i) => new Date(2000, i, 1).toLocaleDateString('en-US', { month: 'long' }))
);
const yearOptions = computed(() => {
  const end = new Date().getFullYear() + 1;
  const list: number[] = [];
  for (let y = 2000; y <= end; y++) list.push(y);
  return list;
});

const monthItems = computed(() => monthOptions.value.map((label, i) => ({ key: String(i), label })));
const yearItems = computed(() => [...yearOptions.value].reverse().map((y) => ({ key: String(y), label: String(y) })));
const monthLabel = computed(() =>
  new Date(viewYear.value, viewMonth.value, 1).toLocaleDateString(undefined, { month: 'long', year: 'numeric' })
);

const cellFor = (value: string) => root.value?.querySelector<HTMLElement>(`[data-day="${value}"]`) ?? null;

async function openCalendar() {
  const base = selectedDate.value || new Date();
  viewYear.value = base.getFullYear();
  viewMonth.value = base.getMonth();
  focusIso.value = iso(base);
  open.value = true;
  await nextTick();
  cellFor(focusIso.value)?.focus();
}

function toggle() {
  if (open.value) open.value = false;
  else void openCalendar();
}

function prevMonth() {
  if (viewMonth.value === 0) {
    viewMonth.value = 11;
    viewYear.value--;
  } else viewMonth.value--;
}
function nextMonth() {
  if (viewMonth.value === 11) {
    viewMonth.value = 0;
    viewYear.value++;
  } else viewMonth.value++;
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

function isToday(d: Date) {
  return iso(d) === iso(new Date());
}
function isSelected(d: Date) {
  const s = selectedDate.value;
  return !!s && iso(s) === iso(d);
}

function selectDate(d: Date) {
  model.value = iso(d);
  open.value = false;
}

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
    <button type="button" :class="buttonCls" aria-haspopup="dialog" :aria-expanded="open" @click="toggle">
      {{ model || placeholder }}
    </button>

    <template v-if="open">
      <div
        ref="root"
        role="dialog"
        :aria-label="`Pick a date, ${monthLabel}`"
        class="absolute left-0 top-full z-50 mt-2 w-72 rounded-card border border-rule bg-page p-3 shadow-lift"
      >
        <div class="mb-2 flex items-center justify-between">
          <button type="button" @click="prevMonth" aria-label="Previous month" class="ns-icon text-ink-soft">
            <ChevronLeft class="w-4 h-4" />
          </button>
          <div class="flex items-center gap-1.5">
            <AppMenu
              :items="monthItems"
              :label="`Month: ${monthOptions[viewMonth]}`"
              align="left"
              @select="viewMonth = Number($event)"
            >
              <template #trigger="{ toggle: toggleMenu, label }">
                <button
                  type="button"
                  class="rounded-control px-1.5 py-1 text-[12px] text-ink-soft hover:bg-page-sunk hover:text-ink"
                  :aria-label="label"
                  @click="toggleMenu"
                >
                  {{ monthOptions[viewMonth] }}
                </button>
              </template>
            </AppMenu>
            <AppMenu :items="yearItems" :label="`Year: ${viewYear}`" align="left" @select="viewYear = Number($event)">
              <template #trigger="{ toggle: toggleMenu, label }">
                <button
                  type="button"
                  class="rounded-control px-1.5 py-1 text-[12px] text-ink-soft hover:bg-page-sunk hover:text-ink"
                  :aria-label="label"
                  @click="toggleMenu"
                >
                  {{ viewYear }}
                </button>
              </template>
            </AppMenu>
          </div>
          <button type="button" @click="nextMonth" aria-label="Next month" class="ns-icon text-ink-soft">
            <ChevronRight class="w-4 h-4" />
          </button>
        </div>

        <div class="grid grid-cols-7 gap-0.5 text-center text-[11px] text-ink-mute mb-1">
          <span v-for="w in ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']" :key="w" class="py-0.5">{{ w }}</span>
        </div>

        <div class="space-y-0.5" role="grid" :aria-label="monthLabel" @keydown="onGridKey">
          <div v-for="(week, i) in weeks" :key="i" class="grid grid-cols-7 gap-0.5" role="row">
            <button
              v-for="(d, j) in week"
              :key="j"
              type="button"
              role="gridcell"
              :disabled="!d"
              :data-day="d ? iso(d) : undefined"
              :tabindex="d && iso(d) === focusIso ? 0 : -1"
              class="w-8 h-8 flex items-center justify-center rounded-control text-[12px] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
              :class="
                d
                  ? isSelected(d)
                    ? 'bg-accent text-on-accent font-semibold hover:bg-accent'
                    : isToday(d)
                      ? 'text-accent font-semibold hover:bg-page-sunk'
                      : 'text-ink hover:bg-page-sunk'
                  : 'invisible'
              "
              :aria-selected="d ? isSelected(d) : undefined"
              :aria-label="d ? readable(d) : undefined"
              @focus="d && (focusIso = iso(d))"
              @click="d && selectDate(d)"
            >
              {{ d?.getDate() }}
            </button>
          </div>
        </div>
      </div>

      <div class="fixed inset-0 z-40" @click="open = false"></div>
    </template>
  </div>
</template>
