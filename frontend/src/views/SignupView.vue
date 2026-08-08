<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/lib/stores/auth';
import AppInput from '@/components/AppInput.vue';
import { Layers } from '@lucide/vue';

const router = useRouter();
const auth = useAuthStore();

const name = ref('');
const email = ref('');
const password = ref('');
const error = ref('');
const loading = ref(false);

async function handleSubmit() {
  error.value = '';
  loading.value = true;
  const success = await auth.signup(email.value, password.value, name.value);
  loading.value = false;
  if (success) {
    router.replace('/');
  } else {
    error.value = 'Signup failed. Email may already be in use.';
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
        <p class="text-text-muted text-lg">Create your account to get started</p>
      </div>

      <div class="card bg-base-200 border border-border shadow-2xl">
        <div class="card-body p-8">
          <h2 class="card-title text-xl font-bold text-white mb-2">Create Account</h2>
          <div v-if="error" class="alert alert-error text-sm py-3 mb-2">
            <span>{{ error }}</span>
          </div>
          <form @submit.prevent="handleSubmit" class="flex flex-col gap-4">
            <AppInput v-model="name" label="Name" placeholder="Your name" required />
            <AppInput v-model="email" label="Email" type="email" placeholder="you@example.com" autocomplete="email" required />
            <AppInput v-model="password" label="Password" type="password" placeholder="At least 6 characters" minlength="6" autocomplete="new-password" required />
            <button type="submit" class="btn btn-primary w-full mt-2" :disabled="loading">
              {{ loading ? 'Creating account...' : 'Create Account' }}
            </button>
          </form>
          <p class="mt-5 text-center text-sm text-subtle">
            Already have an account?
            <router-link to="/login" class="text-primary-400 hover:underline font-medium">Sign in</router-link>
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
