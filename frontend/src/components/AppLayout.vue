<script setup lang="ts">
import { ref } from 'vue';
import { Menu } from '@lucide/vue';
import AppSidebar from '@/components/AppSidebar.vue';

// Mobile drawer state lives here with the burger + overlay it drives; the
// sidebar owns everything else about itself.
const sidebarOpen = ref(false);
</script>

<template>
  <div class="flex h-screen overflow-hidden">
    <button
      class="btn btn-ghost btn-sm md:hidden fixed top-4 left-4 z-[60] text-base-content"
      aria-label="Toggle menu"
      :aria-expanded="sidebarOpen"
      @click="sidebarOpen = !sidebarOpen"
    >
      <Menu class="w-6 h-6" />
    </button>

    <div
      v-if="sidebarOpen"
      class="fixed inset-0 z-[55] bg-black/50 md:hidden"
      @click="sidebarOpen = false"
      aria-hidden="true"
    ></div>

    <AppSidebar :open="sidebarOpen" @navigate="sidebarOpen = false" />

    <div class="flex-1 flex flex-col overflow-hidden" style="background: radial-gradient(ellipse at top, var(--color-bg-elevated) 0%, var(--color-bg) 55%);">
      <main class="flex-1 overflow-auto p-7 w-full">
        <router-view />
      </main>
    </div>
  </div>
</template>
