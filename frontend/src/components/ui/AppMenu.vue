<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';

export interface MenuItem {
  key: string;
  label: string;
  tone?: 'default' | 'danger';
  disabled?: boolean;
}

// A paper panel one level above whatever opens it.
const { align = 'right' } = defineProps<{
  items: MenuItem[];
  align?: 'left' | 'right';
  label: string;
}>();
const emit = defineEmits<{ (e: 'select', key: string): void }>();

const open = ref(false);
const root = ref<HTMLElement | null>(null);
const menuEl = ref<HTMLElement | null>(null);

function trigger(): HTMLElement | null {
  return root.value?.querySelector<HTMLElement>('[aria-haspopup], button') ?? null;
}
function menuItems(): HTMLElement[] {
  if (!menuEl.value) return [];
  return Array.from(menuEl.value.querySelectorAll<HTMLElement>('[role="menuitem"]:not([disabled])'));
}
function focusItem(index: number) {
  const nodes = menuItems();
  if (!nodes.length) return;
  const next = (index + nodes.length) % nodes.length;
  nodes[next].focus();
}

function onDocClick(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) open.value = false;
}
function onDocKeydown(e: KeyboardEvent) {
  if (!open.value) return;
  if (e.key === 'Escape') {
    open.value = false;
    trigger()?.focus();
    return;
  }
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault();
    const nodes = menuItems();
    if (!nodes.length) return;
    const at = nodes.indexOf(document.activeElement as HTMLElement);
    focusItem(at + (e.key === 'ArrowDown' ? 1 : -1));
    return;
  }
  if (e.key === 'Home' || e.key === 'End') {
    e.preventDefault();
    focusItem(e.key === 'Home' ? 0 : -1);
  }
}

watch(open, async (isOpen) => {
  if (isOpen) {
    document.addEventListener('mousedown', onDocClick);
    document.addEventListener('keydown', onDocKeydown);
    await nextTick();
    focusItem(0);
  } else {
    document.removeEventListener('mousedown', onDocClick);
    document.removeEventListener('keydown', onDocKeydown);
  }
});
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocClick);
  document.removeEventListener('keydown', onDocKeydown);
});

function pick(item: MenuItem) {
  if (item.disabled) return;
  emit('select', item.key);
  open.value = false;
  trigger()?.focus();
}
</script>

<template>
  <div ref="root" class="relative">
    <slot name="trigger" :toggle="() => (open = !open)" :open="open" :label="label" />
    <div
      v-if="open"
      ref="menuEl"
      role="menu"
      :aria-label="label"
      class="absolute top-full z-40 mt-1 min-w-[164px] rounded-control border border-rule bg-page p-1 shadow-lift"
      :class="align === 'right' ? 'right-0' : 'left-0'"
    >
      <button
        v-for="item in items"
        :key="item.key"
        type="button"
        role="menuitem"
        :disabled="item.disabled"
        class="flex h-8 w-full cursor-pointer items-center rounded-control px-2.5 text-left text-[13px] transition-colors focus-visible:bg-accent-wash focus-visible:text-ink focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
        :class="
          item.tone === 'danger' ? 'text-debit hover:bg-debit/10' : 'text-ink-soft hover:bg-page-sunk hover:text-ink'
        "
        @click="pick(item)"
      >
        {{ item.label }}
      </button>
    </div>
  </div>
</template>
