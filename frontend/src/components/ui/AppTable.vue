<script setup lang="ts">
export interface Column {
  label: string;
  align?: 'left' | 'right';
  width?: string;
  hideOnMobile?: boolean;
}

// The ledger: a ruled margin, entries, right-aligned numbers, and a carried
// forward line. Every list of money in the app is this table.
withDefaults(defineProps<{ columns: Column[]; loading?: boolean; empty?: boolean; dense?: boolean }>(), {
  loading: false,
  empty: false,
  dense: false
});
</script>

<template>
  <div class="w-full overflow-x-auto">
    <table class="w-full border-collapse text-[13px]">
      <thead>
        <tr>
          <th class="m-cell w-14 border-b border-rule"></th>
          <th
            v-for="col in columns"
            :key="col.label"
            scope="col"
            class="border-b border-rule bg-page-sunk px-3 py-2.5 text-label font-medium uppercase tracking-[0.06em] text-ink-mute"
            :class="[col.align === 'right' ? 'text-right' : 'text-left', col.hideOnMobile ? 'hidden md:table-cell' : '']"
            :style="col.width ? { width: col.width } : undefined"
          >
            {{ col.label }}
          </th>
        </tr>
      </thead>

      <tbody v-if="loading">
        <tr v-for="i in 3" :key="i">
          <td class="m-cell border-b border-rule-soft"></td>
          <td v-for="col in columns" :key="col.label" class="border-b border-rule-soft px-3" :class="dense ? 'py-2' : 'py-3'">
            <span class="block h-2.5 animate-pulse rounded-full bg-page-sunk" :class="col.align === 'right' ? 'ml-auto w-16' : 'w-2/3'" />
          </td>
        </tr>
      </tbody>

      <tbody v-else-if="empty">
        <tr>
          <td :colspan="columns.length + 1" class="px-3 py-5 text-center">
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
/* The margin: a tinted band closed by the steel rule — the signature. */
.m-cell {
  background: color-mix(in oklab, var(--page-sunk) 45%, transparent);
  border-right: 1px solid var(--margin-rule);
  text-align: center;
}
</style>
