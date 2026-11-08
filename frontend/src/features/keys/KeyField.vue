<script setup lang="ts">
// A key as a button that listens for a new one. While it listens every key belongs to it, ahead of the app's own
// keyboard service, and Escape alone gives up.
import { Kbd } from '@glimstone/vue';
import { onBeforeUnmount, ref, watch } from 'vue';
import { comboOf } from '@/core/keymap';
import { keysFirst } from '@/features/config/shared';
import { MODIFIER_KEYS } from './keys';

const props = defineProps<{
  /** The key as it reads, or nothing for none. */
  value?: string;
  /** How many more keys the action has beside the one shown. */
  more?: number;
  listening: boolean;
  listeningText: string;
  emptyText: string;
  /** Why the key does nothing here: it shows dimmed and says so in its tooltip. */
  blocked?: string;
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
    <template v-else-if="value">
      <span :class="{ 'kb-off': blocked }" :data-tip="blocked"><Kbd>{{ value }}</Kbd></span>
      <span v-if="more" class="muted">+{{ more }}</span>
    </template>
    <span v-else class="muted">{{ emptyText }}</span>
  </button>
</template>
