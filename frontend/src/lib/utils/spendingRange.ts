import { toLocalDateString, type SpendingRange } from './format';

/** A date window, both ends inclusive, as `YYYY-MM-DD` strings. */
export interface DateSpan {
  from: string;
  to: string;
}

const DAY = 86400000;

/** The dates a preset covers. `custom` is only read for the CUSTOM range. */
export function resolveSpan(range: SpendingRange, custom: DateSpan, now = new Date()): DateSpan {
  const today = toLocalDateString(now);
  switch (range) {
    case '7D':
      return { from: toLocalDateString(new Date(now.getTime() - 6 * DAY)), to: today };
    case '1M':
      return { from: toLocalDateString(new Date(now.getFullYear(), now.getMonth() - 1, now.getDate())), to: today };
    case '6M':
      return { from: toLocalDateString(new Date(now.getFullYear(), now.getMonth() - 6, now.getDate())), to: today };
    case '1Y':
      return { from: toLocalDateString(new Date(now.getFullYear() - 1, now.getMonth(), now.getDate())), to: today };
    default:
      return { from: custom.from, to: custom.to };
  }
}

/** Whole days in the window, both ends counted. */
export function dayCount(span: DateSpan): number {
  const from = new Date(`${span.from}T00:00:00`);
  const to = new Date(`${span.to}T00:00:00`);
  if (isNaN(from.getTime()) || isNaN(to.getTime())) return 0;
  return Math.max(0, Math.round((to.getTime() - from.getTime()) / DAY)) + 1;
}

/** Five weeks or less is charted by day (a 1M window can be 32 days); longer
 *  windows get a point per month, or the axis turns into a picket fence. */
export function spanGranularity(span: DateSpan): 'day' | 'month' {
  return dayCount(span) <= 35 ? 'day' : 'month';
}

/**
 * `21 Nov – 27 Nov`. The year only appears when the window crosses one, because
 * a two-year range labelled `27 Nov – 27 Nov` would read as a single day.
 */
export function formatSpan(span: DateSpan): string {
  const from = new Date(`${span.from}T00:00:00`);
  const to = new Date(`${span.to}T00:00:00`);
  if (isNaN(from.getTime()) || isNaN(to.getTime())) return `${span.from} – ${span.to}`;
  const day = (d: Date) => `${d.getDate()} ${d.toLocaleDateString(undefined, { month: 'short' })}`;
  if (from.getFullYear() === to.getFullYear()) return `${day(from)} – ${day(to)}`;
  const yy = (d: Date) => String(d.getFullYear()).slice(-2);
  return `${day(from)} ${yy(from)} – ${day(to)} ${yy(to)}`;
}
