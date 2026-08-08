<script setup lang="ts">
import AppInput from '@/components/AppInput.vue';

const form = defineModel<{ name: string }>({ required: true });
</script>

<template>
  <AppInput v-model="form.name" label="Name" placeholder="e.g. Food & Dining" />
</template>
