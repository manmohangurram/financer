<script setup lang="ts">
export interface Column {
  label: string;
  align?: 'left' | 'right';
  width?: string;
  hideOnMobile?: boolean;
}

// Every list of money in the app is this table: tracked-caps head, hairline
// rules, right-aligned figures, optional margin for state glyphs, and a
// carried-forward footer slot.
withDefaults(
  defineProps<{
    columns: Column[];
    loading?: boolean;
    empty?: boolean;
    dense?: boolean;
    margin?: boolean;
    /** sticky head, for long ledgers inside a scroll container */
    sticky?: boolean;
  }>(),
  { loading: false, empty: false, dense: false, margin: true, sticky: false }
);
</script>

<template>
  <div class="w-full overflow-x-auto">
    <table class="table table-sm w-full border-collapse text-[13px]">
      <thead :class="sticky ? 'sticky top-0 z-1 bg-page' : ''">
        <tr>
          <th v-if="margin" class="m-cell w-12 border-b border-rule"></th>
          <th
            v-for="col in columns"
            :key="col.label"
            scope="col"
            class="border-b border-rule px-3 py-2.5 text-label font-medium uppercase tracking-[0.1em] text-ink-mute"
            :class="[
              col.align === 'right' ? 'text-right' : 'text-left',
              col.hideOnMobile ? 'hidden md:table-cell' : ''
            ]"
            :style="col.width ? { width: col.width } : undefined"
          >
            {{ col.label }}
          </th>
        </tr>
      </thead>

      <tbody v-if="loading">
        <tr v-for="i in 3" :key="i">
          <td v-if="margin" class="m-cell border-b border-rule-soft"></td>
          <td
            v-for="col in columns"
            :key="col.label"
            class="border-b border-rule-soft px-3"
            :class="dense ? 'py-2' : 'py-3'"
          >
            <span
              class="skeleton block h-2.5 animate-pulse rounded-[6px] bg-page-sunk"
              :class="col.align === 'right' ? 'ml-auto w-16' : 'w-2/3'"
            />
          </td>
        </tr>
      </tbody>

      <tbody v-else-if="empty">
        <tr>
          <td :colspan="columns.length + (margin ? 1 : 0)" class="px-3 py-5 text-center">
            <slot name="empty" />
          </td>
        </tr>
      </tbody>

      <tbody v-else>
        <slot />
      </tbody>

      <tfoot v-if="$slots.footer">
        <slot name="footer" />
      </tfoot>
    </table>
  </div>
</template>

<style scoped>
/* The margin: a tinted band closed by the accent rule — the one structural
   idea carried from the mark into every table. */
.m-cell {
  background: color-mix(in oklab, var(--page-sunk) 55%, transparent);
  border-right: 1px solid var(--margin-rule);
  text-align: center;
}

:deep(tbody tr:hover td) {
  background: color-mix(in oklab, var(--page-sunk) 45%, transparent);
}
</style>
