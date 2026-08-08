<script setup lang="ts">
import { useRouter } from 'vue-router';
import { ArrowLeft } from '@lucide/vue';

defineProps<{ title: string; subtitle?: string }>();

const router = useRouter();
</script>

<template>
  <div class="flex items-center justify-between gap-4">
    <div class="flex items-center gap-4 min-w-0">
      <button
        aria-label="Back to accounts"
        @click="router.push('/accounts')"
        class="btn btn-ghost btn-sm btn-circle border border-track shrink-0"
      >
        <ArrowLeft class="w-4 h-4" stroke-width="2" />
      </button>
      <div class="min-w-0">
        <h1 class="text-[24px] font-bold text-white tracking-tight">{{ title }}</h1>
        <p v-if="subtitle" class="text-[13.5px] text-subtle mt-0.5 truncate">{{ subtitle }}</p>
      </div>
    </div>
    <div class="flex gap-2 shrink-0">
      <slot name="actions"></slot>
    </div>
  </div>
</template>