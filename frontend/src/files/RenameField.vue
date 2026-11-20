<script setup lang="ts">
// The name as a field in its row, with the name before the extension selected. A name that cannot be taken keeps
// the field and says why in a bubble under it, so the row keeps its height.
import { computeBubblePosition, portalTarget, type BubblePosition } from '@glimstone/core';
import { FieldError } from '@glimstone/vue';
import { nextTick, onMounted, ref, watch } from 'vue';
import { t } from '@/i18n';
import { endInline, inline, saveInline } from './inline';
import { stemLength } from './paths';

const props = defineProps<{ name: string; dir?: boolean }>();
const input = ref<HTMLInputElement | null>(null);
const bubble = ref<HTMLElement | null>(null);
const at = ref<BubblePosition | null>(null);
let done = false;

// The name goes in once, so a render while the reason shows keeps what was typed.
onMounted(() => {
  input.value!.value = props.name;
  input.value!.focus();
  input.value!.setSelectionRange(0, stemLength(props.name, !!props.dir));
});

const toList = () => document.querySelector<HTMLElement>('.panel.active .list')?.focus();

async function finish(keep: boolean) {
  if (done || inline.saving) return;
  if (!keep) {
    done = true;
    endInline();
    toList();
    return;
  }
  await saveInline(input.value!.value);
  if (inline.error) {
    input.value?.focus();
    return;
  }
  done = true;
  toList();
}

function key(e: KeyboardEvent) {
  if (e.key === 'Enter') { e.preventDefault(); void finish(true); }
  if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); void finish(false); }
}

watch(() => inline.error, async (error) => {
  at.value = null;
  if (!error) return;
  await nextTick();
  if (!input.value || !bubble.value) return;
  at.value = computeBubblePosition(input.value.getBoundingClientRect(), { width: bubble.value.offsetWidth, height: bubble.value.offsetHeight }, { width: innerWidth, height: innerHeight });
});
</script>

<template>
  <input ref="input" class="rn-field" :aria-label="t('files.renameField')" :aria-invalid="inline.error ? 'true' : undefined" :aria-describedby="inline.error ? 'rn-error' : undefined"
    spellcheck="false" autocomplete="off" @input="inline.error = ''" @keydown="key" @blur="finish(true)" @click.stop @dblclick.stop @pointerdown.stop @dragstart.prevent>
  <Teleport :to="portalTarget()">
    <div v-if="inline.error" ref="bubble" class="glim-bubble rn-error" :class="{ 'glim-bubble--above': at?.above }" role="alert"
      :style="at ? { left: at.left + 'px', top: at.top + 'px' } : { visibility: 'hidden' }"><FieldError id="rn-error">{{ inline.error }}</FieldError></div>
  </Teleport>
</template>
