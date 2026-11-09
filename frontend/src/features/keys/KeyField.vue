<script setup lang="ts">
// A key as a button that listens for a new one. While it listens every key belongs to it, ahead of the app's own
// keyboard service, and Escape alone gives up.
import { Kbd } from '@glimstone/vue';
import { onBeforeUnmount, ref, watch } from 'vue';
import { comboOf } from '@/core/keymap';
import { keysFirst } from '@/features/config/shared';
import { MODIFIER_KEYS } from './keys';

const props = defineProps<{
  /** Every key the action has, as it reads, each with why it does nothing here if it does not: such a key shows
   *  dimmed and says so in its tooltip. */
  keys: { label: string; blocked?: string }[];
  listening: boolean;
  listeningText: string;
  emptyText: string;
}>();
const emit = defineEmits<{ press: [combo: string]; cancel: [] }>();
const el = ref<HTMLButtonElement | null>(null);

function key(e: KeyboardEvent) {
  e.preventDefault();
  e.stopPropagation();
  if (MODIFIER_KEYS.includes(e.key)) return;
  if (e.key === 'Escape' && !e.ctrlKey && !e.altKey && !e.shiftKey) emit('cancel');
  else emit('press', comboOf(e));
}

let stop: (() => void) | undefined;
watch(() => props.listening, (on) => {
  stop?.();
  stop = on ? keysFirst(key) : undefined;
  if (on) el.value?.focus();
}, { immediate: true, flush: 'post' });
onBeforeUnmount(() => stop?.());
</script>

<template>
  <button ref="el" type="button" class="kb-key" :aria-pressed="listening">
    <span v-if="listening" class="muted">{{ listeningText }}</span>
    <template v-else-if="keys.length">
      <span v-for="k in keys" :key="k.label" :class="{ 'kb-off': k.blocked }" :data-tip="k.blocked"><Kbd>{{ k.label }}</Kbd></span>
    </template>
    <span v-else class="muted">{{ emptyText }}</span>
  </button>
</template>
