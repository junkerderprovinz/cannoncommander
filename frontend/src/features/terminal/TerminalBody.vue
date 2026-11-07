<script setup lang="ts">
// A shell on the server in the window. xterm draws it in the app's colours and monospace font and follows the window's
// size; the shell lives as long as the window, and a dropped connection says so with a way to start again.
import '@xterm/xterm/css/xterm.css';
import { Button } from '@glimstone/vue';
import { FitAddon } from '@xterm/addon-fit';
import { type ITheme, Terminal } from '@xterm/xterm';
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { t } from '@/i18n';
import { type Link, link } from './link';

const props = defineProps<{ path: string; close: () => void }>();
const host = ref<HTMLElement | null>(null);
const state = ref<'connecting' | 'open' | 'ended'>('connecting');
const lost = ref('');

let term: Terminal | null = null;
let fit: FitAddon | null = null;
let wire: Link | null = null;
const watchers: (() => void)[] = [];

// The tokens may be any CSS colour, which xterm cannot read, so a canvas turns each into plain rgb.
function rgb(css: string) {
  const ctx = document.createElement('canvas').getContext('2d');
  if (!ctx || !css) return css;
  ctx.fillStyle = css;
  ctx.fillRect(0, 0, 1, 1);
  const [r, g, b] = ctx.getImageData(0, 0, 1, 1).data;
  return `rgb(${r}, ${g}, ${b})`;
}
// xterm's own colours suit a dark ground. On a light one its yellow and white would vanish, so they get darker.
const LIGHT: ITheme = {
  black: '#1e1e1e', red: '#c4232b', green: '#1f7a33', yellow: '#8a6100', blue: '#1f5fbf', magenta: '#9c2fa8', cyan: '#127a87', white: '#6b6b6b',
  brightBlack: '#5a5a5a', brightRed: '#d63c43', brightGreen: '#2b8f43', brightYellow: '#a67500', brightBlue: '#2f74d6', brightMagenta: '#b443c0', brightCyan: '#16909e', brightWhite: '#3c3c3c',
};
function theme(): ITheme {
  const css = getComputedStyle(host.value ?? document.documentElement);
  const v = (name: string) => rgb(css.getPropertyValue(name).trim());
  const light = document.documentElement.dataset.theme === 'light';
  return {
    ...(light ? LIGHT : {}),
    background: v('--carbon-bg'), foreground: v('--carbon-text'),
    cursor: v('--accent'), cursorAccent: v('--carbon-bg'),
    selectionBackground: v('--accent-soft') || v('--accent'),
  };
}

function connect() {
  state.value = 'connecting';
  lost.value = '';
  wire = link(props.path, term!.cols, term!.rows, {
    open: () => { state.value = 'open'; term!.focus(); },
    data: (bytes) => term!.write(bytes),
    end: (e) => {
      if (e.exited) return props.close();
      state.value = 'ended';
      lost.value = e.text;
    },
  });
}
function again() {
  term!.reset();
  connect();
}

// Copy and paste go as in Windows Terminal: Ctrl+C copies a selection and interrupts without one, Ctrl+V and
// Ctrl+Shift+V paste through the browser, and Ctrl+Shift+C always copies.
function keys(e: KeyboardEvent) {
  if (e.type !== 'keydown' || !(e.ctrlKey || e.metaKey) || e.altKey) return true;
  const key = e.key.toLowerCase(), picked = term!.getSelection();
  if (key === 'c' && (e.shiftKey || picked)) {
    if (picked) void navigator.clipboard?.writeText(picked).catch(() => {});
    e.preventDefault();
    return false;
  }
  return key !== 'v';
}

// The app and the window take Escape and Tab for themselves; inside the terminal both belong to the shell.
function keepKeys(e: KeyboardEvent) {
  if (e.key === 'Escape' && host.value?.contains(e.target as Node)) e.preventDefault();
}

onMounted(() => {
  const el = host.value!;
  term = new Terminal({ fontFamily: getComputedStyle(el).fontFamily, fontSize: 13, cursorBlink: true, scrollback: 5000, theme: theme() });
  fit = new FitAddon();
  term.loadAddon(fit);
  term.open(el);
  fit.fit();
  term.attachCustomKeyEventHandler(keys);
  term.onData((d) => wire?.send(d));
  term.onResize(({ cols, rows }) => wire?.resize(cols, rows));
  const sized = new ResizeObserver(() => fit?.fit());
  sized.observe(el);
  const looks = new MutationObserver(() => { if (term) term.options.theme = theme(); });
  looks.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme', 'style', 'class'] });
  const stop = (e: KeyboardEvent) => e.stopPropagation();
  el.addEventListener('keydown', stop);
  window.addEventListener('keydown', keepKeys, true);
  watchers.push(() => sized.disconnect(), () => looks.disconnect(), () => el.removeEventListener('keydown', stop), () => window.removeEventListener('keydown', keepKeys, true));
  connect();
  term.focus();
});

onBeforeUnmount(() => {
  wire?.close();
  watchers.forEach((fn) => fn());
  term?.dispose();
  term = null;
});
</script>

<template>
  <div class="term-wrap">
    <div ref="host" class="term-host mono" :class="{ dim: state !== 'open' }" data-term></div>
    <p v-if="state === 'connecting'" class="term-note">{{ t('terminal.connecting') }}</p>
    <div v-else-if="state === 'ended'" class="term-note term-lost" role="status">
      <span>{{ lost }}</span>
      <Button data-term-again :label="t('terminal.again')" label-key="IconRefresh" @click="again" />
    </div>
  </div>
</template>
