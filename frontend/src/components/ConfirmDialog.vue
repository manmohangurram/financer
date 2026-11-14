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
      <p class="text-[14px] text-ink-soft">{{ message }}</p>
      <div class="flex justify-end gap-3">
        <button
          type="button"
          @click="emit('cancel')"
          class="ns-quiet border border-rule"
        >
          Cancel
        </button>
        <button
          type="button"
          @click="emit('confirm')"
          class="ns-primary bg-debit hover:bg-debit/90"
        >
          {{ confirmText }}
        </button>
      </div>
    </div>
  </AppModal>
</template>
