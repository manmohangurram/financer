<script setup lang="ts">
import { onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/lib/stores/auth';
import { useRoute } from 'vue-router';

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();

onMounted(async () => {
  await auth.checkAuth();
  if (auth.state.loading) return;
  if (!auth.state.isAuthenticated && route.path !== '/login' && route.path !== '/signup') {
    router.replace('/login');
  }
});
</script>

<template>
  <router-view v-if="!auth.state.loading" />
  <div v-else class="h-full flex items-center justify-center">
    <span class="loading loading-dots loading-lg text-primary"></span>
  </div>
</template>