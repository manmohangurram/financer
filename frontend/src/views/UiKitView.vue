<script setup lang="ts">
// Station 2 proof: every primitive, in every state, on the real tokens.
// Reached at /ui-kit — not linked from the nav; it exists so the system can be
// looked at instead of described.
import { ref } from 'vue';
import { Plus, Trash2, MoreHorizontal } from '@lucide/vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppField from '@/components/ui/AppField.vue';
import AppSelect from '@/components/ui/AppSelect.vue';
import AppStamp from '@/components/ui/AppStamp.vue';
import AppTick from '@/components/ui/AppTick.vue';
import AppTabs from '@/components/ui/AppTabs.vue';
import AppModal from '@/components/ui/AppModal.vue';
import AppMenu from '@/components/ui/AppMenu.vue';
import { toast } from '@/lib/toasts';

const name = ref('');
const note = ref('');
const account = ref('barclays');
const tab = ref('all');
const modalOpen = ref(false);
const tick = ref<'idle' | 'done' | 'refused'>('idle');

const ACCOUNTS = [
  { value: 'barclays', label: 'Barclays ···4412', hint: 'current' },
  { value: 'monzo', label: 'Monzo ···0784', hint: 'savings' },
  { value: 'amex', label: 'Amex ···1009', hint: 'card' }
];
</script>

<template>
  <div class="flex flex-col gap-10">
    <header>
      <h1 class="text-[22px] text-ink">Primitives</h1>
      <p class="mt-1 text-[13px] text-ink-mute">Station 2 — buttons, fields, stamps, ticks, tabs, overlays, toasts.</p>
    </header>

    <section class="flex flex-col gap-3">
      <h2 id="buttons" class="text-[15px]">Buttons</h2>
      <div class="flex flex-wrap items-center gap-2">
        <AppButton>Add entry</AppButton>
        <AppButton variant="outline">Import statement</AppButton>
        <AppButton variant="quiet">Cancel</AppButton>
        <AppButton variant="danger">Delete</AppButton>
        <AppButton variant="icon" label="Add"><Plus class="h-4 w-4" stroke-width="1.8" /></AppButton>
        <AppButton size="sm">Small</AppButton>
        <AppButton disabled>Disabled</AppButton>
        <AppButton loading>Working</AppButton>
      </div>
    </section>

    <section class="flex flex-col gap-3">
      <h2 class="text-[15px]">Fields</h2>
      <div class="grid gap-4 md:grid-cols-3">
        <AppField v-model="name" label="Find entry" placeholder="Merchant, note, amount" hint="Searches name and note." />
        <AppField v-model="name" label="Amount" placeholder="0.00" error="Enter an amount above zero." />
        <AppField v-model="name" label="Locked" placeholder="Not editable" readonly />
        <AppField v-model="name" label="Disabled" placeholder="—" disabled />
        <div class="md:col-span-2">
          <AppField v-model="note" label="Note" multiline placeholder="Why this entry exists" hint="Optional, one line of context." />
        </div>
      </div>
    </section>

    <section class="flex flex-col gap-3">
      <h2 class="text-[15px]">Select</h2>
      <div class="grid gap-4 md:grid-cols-3">
        <AppSelect v-model="account" label="Account" :options="ACCOUNTS" />
        <AppSelect v-model="account" label="Account (error)" :options="ACCOUNTS" error="Pick the account this belongs to." />
        <AppSelect :model-value="''" label="Empty" :options="[]" placeholder="Nothing available" />
      </div>
    </section>

    <section class="flex flex-col gap-3">
      <h2 class="text-[15px]">Stamps &amp; ticks</h2>
      <div class="flex flex-wrap items-center gap-2">
        <AppStamp>···4412</AppStamp>
        <AppStamp tone="pending">pending</AppStamp>
        <AppStamp tone="accent">auto</AppStamp>
        <AppStamp tone="credit">paid</AppStamp>
        <AppStamp tone="debit">failed</AppStamp>
        <span class="mx-3 h-6 w-px bg-rule"></span>
        <AppTick :state="tick" label="Approve" @click="tick = tick === 'done' ? 'idle' : 'done'" />
        <AppTick state="done" label="Approved" />
        <AppTick state="refused" label="Refused" />
      </div>
      <p class="text-[12px] text-ink-mute">Click the dashed tick — the state is what the Inbox queue does.</p>
    </section>

    <section class="flex flex-col gap-3">
      <h2 class="text-[15px]">Tabs</h2>
      <AppTabs
        v-model="tab"
        :items="[
          { key: 'all', label: 'All', count: 128 },
          { key: 'review', label: 'To review', count: 4 },
          { key: 'auto', label: 'Booked' }
        ]"
      />
    </section>

    <section class="flex flex-col gap-3">
      <h2 class="text-[15px]">Overlays</h2>
      <div class="flex flex-wrap items-center gap-2">
        <AppButton variant="outline" @click="modalOpen = true">Open modal</AppButton>
        <AppMenu
          label="Row actions"
          :items="[
            { key: 'edit', label: 'Edit' },
            { key: 'duplicate', label: 'Duplicate' },
            { key: 'delete', label: 'Delete', tone: 'danger' }
          ]"
          @select="(k) => toast(`Menu: ${k}`)"
        >
          <template #trigger="{ toggle }">
            <AppButton variant="icon" label="Row actions" @click="toggle">
              <MoreHorizontal class="h-4 w-4" stroke-width="1.8" />
            </AppButton>
          </template>
        </AppMenu>
        <AppButton variant="quiet" @click="toast('Transaction booked', { action: { label: 'Undo', run: () => toast('Undone') } })">
          Show toast
        </AppButton>
        <AppButton variant="quiet" @click="toast('That did not save', { tone: 'error' })">Show error toast</AppButton>
      </div>
    </section>

    <AppModal :open="modalOpen" title="Link an entry" @close="modalOpen = false">
      <div class="flex flex-col gap-4">
        <AppField v-model="name" label="Merchant" placeholder="Sainsbury's" />
        <AppSelect v-model="account" label="Account" :options="ACCOUNTS" />
      </div>
      <template #footer>
        <AppButton variant="quiet" @click="modalOpen = false">Cancel</AppButton>
        <AppButton @click="modalOpen = false">Save entry</AppButton>
      </template>
    </AppModal>

    <section class="flex flex-col gap-3">
      <h2 class="text-[15px]">Type</h2>
      <div class="flex flex-col gap-2 border-t border-rule pt-4">
        <span class="font-display text-[36px] leading-none tracking-[-0.02em] text-ink">£41,286.40</span>
        <span class="font-display text-[22px] text-ink">Page title</span>
        <span class="text-[15px] font-semibold text-ink">Section title</span>
        <span class="text-[14px] text-ink">Body copy at the size the whole app runs at.</span>
        <span class="text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Carried forward</span>
        <span class="tabular text-[13px] text-ink-soft">+4,120.00 · −128.44 · −8.75</span>
        <span class="font-mono text-[12px] text-ink-mute">fin_7c3a9e21 · polled 12m</span>
      </div>
    </section>

    <p class="flex items-center gap-2 text-[12px] text-ink-mute">
      <Trash2 class="h-[13px] w-[13px]" stroke-width="1.6" aria-hidden="true" />
      Every control above is the shipped component, not a mock.
    </p>
  </div>
</template>
