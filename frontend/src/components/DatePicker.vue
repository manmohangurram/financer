<script setup lang="ts">
import { ref, computed } from 'vue';
import { ChevronLeft, ChevronRight } from '@lucide/vue';

const model = defineModel<string | undefined>({ required: true });
const props = withDefaults(defineProps<{ placeholder?: string; class?: string }>(), { placeholder: 'Pick a date' });

const open = ref(false);
const viewYear = ref(new Date().getFullYear());
const viewMonth = ref(new Date().getMonth());

const buttonCls = computed(() =>
  ['input w-full bg-surface text-left', model.value ? 'text-text' : 'text-text-muted', props.class].join(' ')
);

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

function openCalendar() {
  const base = selectedDate.value || new Date();
  viewYear.value = base.getFullYear();
  viewMonth.value = base.getMonth();
  open.value = true;
}

function prevMonth() {
  if (viewMonth.value === 0) { viewMonth.value = 11; viewYear.value--; }
  else viewMonth.value--;
}
function nextMonth() {
  if (viewMonth.value === 11) { viewMonth.value = 0; viewYear.value++; }
  else viewMonth.value++;
}

function isToday(d: Date) {
  const n = new Date();
  return d.getFullYear() === n.getFullYear() && d.getMonth() === n.getMonth() && d.getDate() === n.getDate();
}
function isSelected(d: Date) {
  const s = selectedDate.value;
  return !!s && s.getFullYear() === d.getFullYear() && s.getMonth() === d.getMonth() && s.getDate() === d.getDate();
}

function selectDate(d: Date) {
  model.value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  open.value = false;
}
</script>

<template>
  <div class="relative">
    <button
      type="button"
      :class="buttonCls"
      @click="open ? (open = false) : openCalendar()"
    >
      {{ model || placeholder }}
    </button>

    <div
      v-if="open"
      class="absolute left-0 top-full z-50 mt-2 bg-base-100 border border-base-300 rounded-box shadow-lg p-3 w-72"
    >
      <div class="flex items-center justify-between mb-2">
        <button type="button" @click="prevMonth" aria-label="Previous month" class="btn btn-ghost btn-xs text-text-muted">
          <ChevronLeft class="w-4 h-4" />
        </button>
        <div class="flex items-center gap-1.5">
          <select v-model.number="viewMonth" class="select select-xs bg-surface border-border text-text" aria-label="Select month">
            <option v-for="(m, i) in monthOptions" :key="i" :value="i">{{ m }}</option>
          </select>
          <select v-model.number="viewYear" class="select select-xs bg-surface border-border text-text" aria-label="Select year">
            <option v-for="y in yearOptions" :key="y" :value="y">{{ y }}</option>
          </select>
        </div>
        <button type="button" @click="nextMonth" aria-label="Next month" class="btn btn-ghost btn-xs text-text-muted">
          <ChevronRight class="w-4 h-4" />
        </button>
      </div>

      <div class="grid grid-cols-7 gap-0.5 text-center text-[11px] text-subtle mb-1">
        <span v-for="w in ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']" :key="w" class="py-0.5">{{ w }}</span>
      </div>

      <div class="space-y-0.5">
        <div v-for="(week, i) in weeks" :key="i" class="grid grid-cols-7 gap-0.5">
          <button
            v-for="(d, j) in week"
            :key="j"
            type="button"
            :disabled="!d"
            class="w-8 h-8 flex items-center justify-center rounded-md text-[12px] transition-colors"
            :class="
              d
                ? isSelected(d)
                  ? 'bg-primary-600 text-white font-semibold'
                  : isToday(d)
                    ? 'text-primary-400 font-semibold'
                    : 'text-text hover:bg-white/5'
                : ''
            "
            @click="d && selectDate(d)"
          >{{ d?.getDate() }}</button>
        </div>
      </div>
    </div>

    <div v-if="open" class="fixed inset-0 z-40" @click="open = false"></div>
  </div>
</template>