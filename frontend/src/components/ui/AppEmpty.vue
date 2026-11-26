<script setup lang="ts">
// An empty surface states the fact and the next action in one place. No
// illustration, no mascot: one line, one hint, one button.
defineProps<{ title: string; hint?: string }>();
</script>

<template>
  <div class="flex flex-col items-center gap-2 py-8 text-center">
    <slot name="icon" />
    <p class="text-[13.5px] font-medium text-ink">{{ title }}</p>
    <p v-if="hint" class="max-w-[46ch] text-[12px] leading-relaxed text-ink-mute">
      {{ hint }}
    </p>
    <div v-if="$slots.action" class="mt-2">
      <slot name="action" />
    </div>
  </div>
</template>
