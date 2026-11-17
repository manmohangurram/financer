<script setup lang="ts">
// The one stepper in the product: numbered circles, hairline connectors, ✓ on
// completed steps. Used by the rule modal and the statement import.
defineProps<{ steps: string[]; current: number }>();
const emit = defineEmits<{ (e: 'select', index: number): void }>();
</script>

<template>
  <ol class="flex shrink-0 items-center border-b border-rule pb-3.5" aria-label="Steps">
    <li
      v-for="(label, index) in steps"
      :key="label"
      class="flex items-center"
      :class="index < steps.length - 1 ? 'flex-1' : ''"
    >
      <button
        type="button"
        class="flex shrink-0 items-center gap-2 rounded-control px-1 py-0.5 text-[12.5px] transition-colors"
        :class="index === current ? 'font-semibold text-ink' : index < current ? 'text-ink-soft hover:text-ink' : 'text-ink-mute'"
        :disabled="index > current"
        :aria-current="index === current ? 'step' : undefined"
        @click="emit('select', index)"
      >
        <span
          class="grid h-[22px] w-[22px] place-items-center rounded-full border text-[11px]"
          :class="index === current ? 'border-accent bg-accent text-page' : index < current ? 'border-accent text-accent' : 'border-rule-strong'"
        >{{ index < current ? '✓' : index + 1 }}</span>
        {{ label }}
      </button>
      <span v-if="index < steps.length - 1" class="mx-2.5 h-px flex-1 bg-rule" aria-hidden="true" />
    </li>
  </ol>
</template>
