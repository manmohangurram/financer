<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/lib/stores/auth';
import AppButton from '@/components/ui/AppButton.vue';
import AppField from '@/components/ui/AppField.vue';

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
  if (success) router.replace('/');
  else error.value = 'That email is already in use, or the password is too short.';
}
</script>

<template>
  <main class="grid min-h-dvh place-items-center px-4 py-10">
    <div class="w-full max-w-[400px]">
      <div class="mb-7 flex items-center gap-3">
        <svg viewBox="0 0 64 64" class="h-9 w-9 text-ink" aria-hidden="true">
          <rect x="8" y="5" width="48" height="54" rx="5" fill="none" stroke="currentColor" stroke-width="4.5" />
          <rect x="12" y="9" width="13" height="46" rx="2" fill="currentColor" />
          <polyline
            points="14,31 17.5,35 22,25.5"
            fill="none"
            stroke="var(--page)"
            stroke-width="4"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <line x1="30" y1="20" x2="50" y2="20" stroke="currentColor" stroke-width="3" />
          <line x1="30" y1="32" x2="44" y2="32" stroke="currentColor" stroke-width="3" />
          <line x1="30" y1="44" x2="47" y2="44" stroke="currentColor" stroke-width="3" />
        </svg>
        <span class="font-display text-[24px] leading-none tracking-[-0.02em] text-ink">financer</span>
      </div>

      <div class="card rounded-card border border-rule bg-page p-6 shadow-lift md:p-7">
        <h1 class="text-[22px] leading-none text-ink">Create your ledger</h1>
        <p class="mt-1.5 text-[13px] text-ink-mute">One book per person. Nothing is shared.</p>

        <form class="mt-6 flex flex-col gap-4" @submit.prevent="handleSubmit">
          <AppField v-model="name" label="Name" placeholder="Your name" autocomplete="name" />
          <AppField v-model="email" label="Email" type="email" placeholder="you@example.com" autocomplete="email" />
          <AppField
            v-model="password"
            label="Password"
            type="password"
            placeholder="At least 6 characters"
            hint="Six characters or more."
            autocomplete="new-password"
          />
          <p
            v-if="error"
            role="alert"
            class="rounded-control border border-debit/40 px-3 py-2 text-[12.5px] text-debit"
          >
            {{ error }}
          </p>
          <AppButton type="submit" :loading="loading" class="w-full">Create account</AppButton>
        </form>

        <p class="mt-5 border-t border-rule-soft pt-4 text-[12.5px] text-ink-mute">
          Already have one?
          <router-link to="/login" class="font-medium text-accent hover:text-accent-hover">Sign in</router-link>
        </p>
      </div>
    </div>
  </main>
</template>
