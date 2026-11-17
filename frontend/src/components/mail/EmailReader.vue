<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { ChevronDown, Code2, Loader2 } from '@lucide/vue';
import { formatDate } from '@/lib/utils/format';
import type { MailMessageDetail } from '@/lib/types/mail';

const props = defineProps<{
  message: MailMessageDetail | null;
  mailbox: string;
  loading?: boolean;
}>();

const showSource = ref(false);
watch(
  () => props.message?.id,
  () => {
    showSource.value = false;
  }
);

const sender = computed(() => props.message?.fromName || props.message?.fromAddr || 'Unknown sender');
const paragraphs = computed(() => {
  const body = props.message?.bodyText || '';
  return body.split(/\n{2,}/).map((block) => block.trim()).filter(Boolean);
});

function stampClass(status: string): string {
  if (status === 'AUTO') return 'badge-success';
  if (status === 'UNPARSED') return 'badge-error';
  if (status === 'PENDING') return 'badge-warning';
  return '';
}
</script>

<template>
  <article v-if="message" class="flex flex-col">
    <header class="border-b border-rule-soft px-5 py-4 md:px-7">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <p class="flex flex-wrap items-center gap-2 text-[12.5px] text-ink-mute">
            <span class="font-medium text-ink">{{ sender }}</span>
            <span class="text-ink-faint">&lt;{{ message.fromAddr }}&gt;</span>
          </p>
          <h2 class="mt-1 font-display text-[22px] leading-tight text-ink">
            {{ message.subject || '(no subject)' }}
          </h2>
          <p class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-[12px] text-ink-mute">
            <span>{{ message.receivedAt ? formatDate(message.receivedAt) : '—' }}</span>
            <span aria-hidden="true">·</span>
            <span>{{ mailbox }}</span>
            <span v-if="message.confidence" aria-hidden="true">·</span>
            <span v-if="message.confidence" class="mono">{{ Math.round(message.confidence * 100) }}% parse</span>
          </p>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="badge badge-sm badge-outline" :class="stampClass(message.status)">{{ message.status }}</span>
        </div>
      </div>
    </header>

    <div class="px-5 py-5 md:px-7">
      <div v-if="loading" class="flex items-center gap-2 py-6 text-[13px] text-ink-mute">
        <Loader2 class="h-4 w-4 animate-spin" /> Reading the message…
      </div>

      <div v-else class="max-w-[76ch]">
        <p class="mb-2 text-label font-medium uppercase tracking-[0.06em] text-ink-mute">Message</p>
        <div class="email-body email-scroll h-[260px] overflow-y-auto pr-2 md:h-[340px]">
          <p v-for="(block, i) in paragraphs" :key="i" class="whitespace-pre-wrap">{{ block }}</p>
          <p v-if="!paragraphs.length" class="text-ink-mute">No readable content in this message.</p>
        </div>
      </div>

      <div class="mt-5 border-t border-rule-soft pt-3">
        <button
          type="button"
          class="btn btn-ghost btn-xs gap-1.5 px-1 text-ink-mute hover:text-ink"
          :aria-expanded="showSource"
          @click="showSource = !showSource"
        >
          <Code2 class="h-3.5 w-3.5" stroke-width="1.7" aria-hidden="true" />
          Original message
          <ChevronDown
            class="h-3.5 w-3.5 transition-transform"
            :class="showSource ? 'rotate-180' : ''"
            stroke-width="1.7"
            aria-hidden="true"
          />
        </button>
        <pre
          v-if="showSource"
          class="mt-2 max-h-[320px] overflow-auto whitespace-pre-wrap rounded-control border border-rule-soft bg-page-sunk p-3 font-mono text-[11.5px] leading-relaxed text-ink-soft"
          >{{ message.rawText || 'No source stored.' }}</pre
        >
      </div>
    </div>
  </article>

  <div v-else class="flex flex-col items-center justify-center gap-2 px-6 py-16 text-center">
    <p class="text-[14px] text-ink">Select a mail to review</p>
    <p class="max-w-[36ch] text-[12.5px] text-ink-mute">
      The email appears here with its booking fields underneath.
    </p>
  </div>
</template>

<style scoped>
.email-body p {
  font-size: 14px;
  line-height: 1.75;
  color: var(--ink-soft);
}
.email-body p + p {
  margin-top: 12px;
}
/* The message has its own fixed viewport; the reader text never pushes the
   booking form off screen. */
.email-scroll {
  scrollbar-width: thin;
  scrollbar-color: var(--rule-strong) transparent;
  scrollbar-gutter: stable;
}
.email-scroll::-webkit-scrollbar {
  width: 10px;
}
.email-scroll::-webkit-scrollbar-track {
  background: transparent;
}
.email-scroll::-webkit-scrollbar-thumb {
  background: var(--rule-strong);
  border: 3px solid transparent;
  border-radius: 999px;
  background-clip: content-box;
}
.email-scroll::-webkit-scrollbar-thumb:hover {
  background: var(--ink-faint);
  background-clip: content-box;
}
.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
</style>
