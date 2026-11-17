<script setup lang="ts">
import AppInput from '@/components/AppInput.vue';

const form = defineModel<{ name: string; monthlyBudget: string }>({ required: true });
</script>

<template>
  <div class="flex flex-col gap-4">
    <AppInput v-model="form.name" label="Name" placeholder="e.g. Food & Dining" />
    <AppInput
      v-model="form.monthlyBudget"
      label="Monthly budget"
      type="number"
      min="0"
      step="0.01"
      placeholder="No budget"
      hint="Optional. Repeats every month; applies to debit spend only."
    />
  </div>
</template>
