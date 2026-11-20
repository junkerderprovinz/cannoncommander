<script setup lang="ts">
import { IconClose, IconTipButton, IconWindowMaximise, IconWindowMinimise, IconWindowRestore } from '@glimstone/vue';
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { t } from '@/i18n';
import { frame, resizeFromEdges } from './frame';

// The desktop app's window buttons where its window has no frame, at the end of the head with the head's own gap. The
// head's free space and the app's edge around it move the window, as a title bar would.

// How long the page has to stop resizing before the window is asked whether it is maximised.
const SETTLE_MS = 100;

// Maximising and restoring resize the page whatever started them, the buttons, a snap or a key, so a resize that has
// settled is the cue to ask.
const maximised = ref(false);
let timer: ReturnType<typeof setTimeout> | undefined;
const ask = () => frame.maximised().then((m) => { maximised.value = m; }, () => {});
const settle = () => {
  clearTimeout(timer);
  timer = setTimeout(ask, SETTLE_MS);
};

// A maximised window has no edge to pull, and its corner belongs to close.
let edges: (() => void) | undefined;
watch(maximised, (m) => {
  edges?.();
  edges = m ? undefined : resizeFromEdges();
}, { immediate: true });

const quiet = (p: Promise<unknown>) => { p.catch(() => {}); };
const toggle = () => quiet(frame.setMaximised(!maximised.value));

// Only a press on the head or the app themselves moves the window, never one on something in them. The drag starts
// on the first move, so a click and a double click stay clicks. An edge the resize has taken never gets here.
const free = (e: MouseEvent) => (e.target as Element).matches?.('.head, .app');
let pressed = false;
const down = (e: MouseEvent) => { pressed = e.button === 0 && free(e); };
const up = () => { pressed = false; };
function move(e: MouseEvent) {
  if (!pressed) return;
  pressed = false;
  if (e.buttons === 1) frame.drag();
}
const twice = (e: MouseEvent) => { if (free(e)) toggle(); };

onMounted(() => {
  void ask();
  addEventListener('resize', settle);
  document.addEventListener('mousedown', down);
  document.addEventListener('mousemove', move);
  document.addEventListener('mouseup', up);
  document.addEventListener('dblclick', twice);
});
onBeforeUnmount(() => {
  removeEventListener('resize', settle);
  document.removeEventListener('mousedown', down);
  document.removeEventListener('mousemove', move);
  document.removeEventListener('mouseup', up);
  document.removeEventListener('dblclick', twice);
  clearTimeout(timer);
  edges?.();
});
</script>

<template>
  <span class="window-controls">
    <IconTipButton class="glim-window-button" :tip="t('desktop.minimise')" @click="quiet(frame.minimise())">
      <span class="glim-window-square"><IconWindowMinimise /></span>
    </IconTipButton>
    <IconTipButton class="glim-window-button" :tip="t(maximised ? 'desktop.restore' : 'desktop.maximise')" @click="toggle">
      <span class="glim-window-square"><IconWindowRestore v-if="maximised" /><IconWindowMaximise v-else /></span>
    </IconTipButton>
    <IconTipButton class="glim-window-button" :tip="t('core.close')" @click="quiet(frame.close())">
      <span class="glim-window-square"><IconClose /></span>
    </IconTipButton>
  </span>
</template>
