<script setup lang="ts">
import AppModal from '@/components/AppModal.vue';

withDefaults(defineProps<{ title?: string; message: string; confirmText?: string }>(), {
  title: 'Confirm',
  confirmText: 'Delete'
});
const emit = defineEmits<{ (e: 'confirm'): void; (e: 'cancel'): void }>();
</script>

<template>
  <AppModal :title="title" @close="emit('cancel')">
    <div class="p-6 space-y-5">
      <p class="text-[14px] text-text-secondary">{{ message }}</p>
      <div class="flex justify-end gap-3">
        <button
          type="button"
          @click="emit('cancel')"
          class="px-4 py-2 rounded-lg text-[13px] font-medium text-text-muted border border-border hover:border-primary-500/40 transition-colors"
        >
          Cancel
        </button>
        <button
          type="button"
          @click="emit('confirm')"
          class="px-4 py-2 rounded-lg text-[13px] font-medium bg-expense text-white hover:bg-expense/80 transition-colors"
        >
          {{ confirmText }}
        </button>
      </div>
    </div>
  </AppModal>
</template>
