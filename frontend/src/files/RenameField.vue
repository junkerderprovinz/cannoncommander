<script setup lang="ts">
// The name as a field in its row, with the name before the extension selected.
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';
import { endInline, saveInline, stemLength } from './inline';

const props = defineProps<{ name: string; dir?: boolean }>();
const input = ref<HTMLInputElement | null>(null);
let done = false;

onMounted(() => {
  const el = input.value!;
  el.focus();
  el.setSelectionRange(0, stemLength(props.name, !!props.dir));
});

function finish(keep: boolean) {
  if (done) return;
  done = true;
  if (keep) void saveInline(input.value!.value);
  else endInline();
  document.querySelector<HTMLElement>('.panel.active .list')?.focus();
}

function key(e: KeyboardEvent) {
  if (e.key === 'Enter') { e.preventDefault(); finish(true); }
  if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); finish(false); }
}
</script>

<template>
  <input ref="input" class="rn-field" :value="name" :aria-label="t('files.renameField')" spellcheck="false" autocomplete="off"
    @keydown="key" @blur="finish(true)" @click.stop @dblclick.stop @pointerdown.stop @dragstart.prevent>
</template>
