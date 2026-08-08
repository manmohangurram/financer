<script setup lang="ts">
defineProps<{ title: string; wide?: boolean; xwide?: boolean }>();
const emit = defineEmits<{ (e: 'close'): void }>();
</script>

<template>
  <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60" @click.self="emit('close')">
    <div
      class="w-full rounded-2xl bg-gradient-to-br from-surface to-surface-alt border border-border"
      :class="xwide ? 'max-w-4xl' : wide ? 'max-w-2xl' : 'max-w-lg'"
    >
      <div class="flex items-center justify-between px-6 py-5 border-b border-border">
        <h3 class="text-lg font-semibold text-text">{{ title }}</h3>
        <button @click="emit('close')" class="text-subtle hover:text-text-secondary text-xl">&times;</button>
      </div>
      <slot />
    </div>
  </div>
</template>
