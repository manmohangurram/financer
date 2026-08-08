<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/lib/stores/auth';
import AppInput from '@/components/AppInput.vue';
import { Layers } from '@lucide/vue';

const router = useRouter();
const auth = useAuthStore();

const email = ref('');
const password = ref('');
const error = ref('');
const loading = ref(false);

async function handleSubmit() {
  error.value = '';
  loading.value = true;
  const success = await auth.login(email.value, password.value);
  loading.value = false;
  if (success) {
    router.replace('/');
  } else {
    error.value = 'Invalid email or password';
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center p-4" style="background: radial-gradient(ellipse at 30% 20%, var(--color-bg-auth) 0%, var(--color-bg) 60%);">
    <div class="w-full max-w-md">
      <div class="text-center mb-10">
        <div class="flex items-center justify-center gap-4 mb-5">
          <div class="w-[52px] h-[52px] rounded-3xl bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center text-2xl">
            <Layers class="w-6 h-6 text-white" stroke-width="2" />
          </div>
          <div class="text-left">
            <div class="text-[28px] font-extrabold text-white tracking-tight">Financer</div>
            <div class="text-sm text-subtle font-medium">Personal Finance</div>
          </div>
        </div>
        <p class="text-text-muted text-lg">Sign in to manage your finances</p>
      </div>

      <div class="card bg-base-200 border border-border shadow-2xl">
        <div class="card-body p-8">
          <h2 class="card-title text-xl font-bold text-white mb-2">Sign In</h2>
          <div v-if="error" class="alert alert-error text-sm py-3 mb-2">
            <span>{{ error }}</span>
          </div>
          <form @submit.prevent="handleSubmit" class="flex flex-col gap-4">
            <AppInput v-model="email" label="Email" type="email" placeholder="Enter your email" autocomplete="email" required />
            <AppInput v-model="password" label="Password" type="password" placeholder="Enter your password" autocomplete="current-password" required />
            <button type="submit" class="btn btn-primary w-full mt-2" :disabled="loading">
              {{ loading ? 'Signing in...' : 'Sign In' }}
            </button>
          </form>
          <p class="mt-5 text-center text-sm text-subtle">
            Don't have an account?
            <router-link to="/signup" class="text-primary-400 hover:underline font-medium">Sign up</router-link>
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
