<script setup lang="ts">
import { ChevronDown } from '@lucide/vue';

// A native <select> on Nocturne tokens: the browser keeps its own menu and
// keyboard behaviour, only the closed field is restyled. Use ui/AppSelect when
// the options need hints or a searchable list.
defineProps<{ label?: string; hint?: string }>();
const model = defineModel<string | number>();
</script>

<template>
  <label class="flex w-full flex-col gap-1.5">
    <span v-if="label" class="text-label font-medium uppercase tracking-[0.1em] text-ink-mute">{{ label }}</span>
    <span class="relative block">
      <select
        v-model="model"
        class="h-[38px] w-full appearance-none rounded-control border border-rule bg-page-sunk pl-3 pr-8 text-[14px] text-ink transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20"
      >
        <slot />
      </select>
      <ChevronDown
        class="pointer-events-none absolute right-2.5 top-1/2 h-[15px] w-[15px] -translate-y-1/2 text-ink-mute"
        stroke-width="1.6"
        aria-hidden="true"
      />
    </span>
    <span v-if="hint" class="text-[12px] text-ink-mute">{{ hint }}</span>
  </label>
</template>
