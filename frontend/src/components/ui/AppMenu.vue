<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue';

export interface MenuItem {
  key: string;
  label: string;
  tone?: 'default' | 'danger';
  disabled?: boolean;
}

// A paper panel one level above whatever opens it.
const { align = 'right' } = defineProps<{ items: MenuItem[]; align?: 'left' | 'right'; label: string }>();
const emit = defineEmits<{ (e: 'select', key: string): void }>();

const open = ref(false);
const root = ref<HTMLElement | null>(null);

function onDocClick(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) open.value = false;
}
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false;
}

watch(open, (isOpen) => {
  if (isOpen) {
    document.addEventListener('mousedown', onDocClick);
    document.addEventListener('keydown', onKeydown);
  } else {
    document.removeEventListener('mousedown', onDocClick);
    document.removeEventListener('keydown', onKeydown);
  }
});
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocClick);
  document.removeEventListener('keydown', onKeydown);
});

function pick(item: MenuItem) {
  if (item.disabled) return;
  emit('select', item.key);
  open.value = false;
}
</script>

<template>
  <div ref="root" class="relative">
    <slot name="trigger" :toggle="() => (open = !open)" :open="open" :label="label" />
    <div
      v-if="open"
      role="menu"
      class="absolute top-full z-40 mt-1 min-w-[164px] rounded-card border border-rule bg-page p-1 shadow-lift"
      :class="align === 'right' ? 'right-0' : 'left-0'"
    >
      <button
        v-for="item in items"
        :key="item.key"
        type="button"
        role="menuitem"
        :disabled="item.disabled"
        class="flex w-full items-center rounded-control px-2.5 py-1.5 text-left text-[13px] transition-colors disabled:opacity-40"
        :class="item.tone === 'danger' ? 'text-debit hover:bg-debit/10' : 'text-ink-soft hover:bg-page-sunk hover:text-ink'"
        @click="pick(item)"
      >
        {{ item.label }}
      </button>
    </div>
  </div>
</template>
