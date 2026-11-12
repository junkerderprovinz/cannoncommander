<script setup lang="ts">
import { IconClose, IconTipButton, IconWindowMaximise, IconWindowMinimise, IconWindowRestore } from '@glimstone/vue';
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { t } from '@/i18n';
import { frame, resizeFromEdges } from './frame';

// The title bar of the desktop app's window where it has no native one, as GlimStone's WindowBar draws it: a strip to
// drag and the three window buttons. The head already shows the mark and the name.

// How long the page has to stop resizing before the window is asked whether it is maximised.
const SETTLE_MS = 100;

// Maximising and restoring resize the page whatever started them, the bar, a snap or a key, so a resize that has
// settled is the cue to ask.
const maximised = ref(false);
let timer: ReturnType<typeof setTimeout> | undefined;
const ask = () => frame.maximised().then((m) => { maximised.value = m; }, () => {});
const settle = () => {
  clearTimeout(timer);
  timer = setTimeout(ask, SETTLE_MS);
};
onMounted(() => {
  void ask();
  addEventListener('resize', settle);
});

// A maximised window has no edge to pull, and its corner belongs to close.
let edges: (() => void) | undefined;
watch(maximised, (m) => {
  edges?.();
  edges = m ? undefined : resizeFromEdges();
}, { immediate: true });

onBeforeUnmount(() => {
  removeEventListener('resize', settle);
  clearTimeout(timer);
  edges?.();
});

const quiet = (p: Promise<unknown>) => { p.catch(() => {}); };
const toggle = () => quiet(frame.setMaximised(!maximised.value));

// The drag starts on the first move, not on the press, so a click and a double click on the bar stay clicks. An edge
// the resize has taken never reaches the bar.
let pressed = false;
const onButton = (e: MouseEvent) => (e.target as Element).closest('button') !== null;
const down = (e: MouseEvent) => { pressed = e.button === 0 && !onButton(e); };
const up = () => { pressed = false; };
function move(e: MouseEvent) {
  if (!pressed) return;
  pressed = false;
  if (e.buttons === 1) frame.drag();
}
const twice = (e: MouseEvent) => { if (!onButton(e)) toggle(); };
</script>

<template>
  <header aria-label="CannonCommander" class="glim-window-bar" @mousedown="down" @mousemove="move" @mouseup="up" @dblclick="twice">
    <IconTipButton class="glim-window-button" :tip="t('desktop.minimise')" @click="quiet(frame.minimise())">
      <span class="glim-window-square"><IconWindowMinimise /></span>
    </IconTipButton>
    <IconTipButton class="glim-window-button" :tip="t(maximised ? 'desktop.restore' : 'desktop.maximise')" @click="toggle">
      <span class="glim-window-square"><IconWindowRestore v-if="maximised" /><IconWindowMaximise v-else /></span>
    </IconTipButton>
    <IconTipButton class="glim-window-button" :tip="t('core.close')" @click="quiet(frame.close())">
      <span class="glim-window-square"><IconClose /></span>
    </IconTipButton>
  </header>
</template>
