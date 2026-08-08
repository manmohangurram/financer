<script setup lang="ts">
withDefaults(defineProps<{ panelClass?: string; up?: boolean }>(), { panelClass: 'right-0 min-w-40', up: false });
const emit = defineEmits<{ (e: 'close'): void }>();
</script>

<template>
  <div class="absolute inset-0 z-50">
    <div class="fixed inset-0 z-40" @click="emit('close')"></div>
    <div class="absolute z-50" :class="[up ? 'bottom-full mb-2' : 'top-full mt-2', panelClass]">
      <div class="bg-surface border border-border rounded-xl shadow-popover overflow-hidden" @click.stop>
        <slot />
      </div>
    </div>
  </div>
</template>
