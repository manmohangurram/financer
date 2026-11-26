<script setup lang="ts">
import AppModal from '@/components/AppModal.vue';
import AppButton from '@/components/ui/AppButton.vue';

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
        <AppButton variant="outline" @click="emit('cancel')">Cancel</AppButton>
        <AppButton variant="dangerSolid" @click="emit('confirm')">{{ confirmText }}</AppButton>
      </div>
    </div>
  </AppModal>
</template>
