<script setup lang="ts">
// The body of the F3 window: one renderer, large, with the file's line in the foot.
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { fmtSize, fmtTime } from '@/core/format';
import type { WinEntry } from '@/core/windows';
import { t } from '@/i18n';
import type { FileNode } from './kinds';
import type { KeyHandler } from './renderers/types';
import Stage from './Stage.vue';
import { footStartOf, winKeys } from './winkeys';

const props = defineProps<{ node: FileNode; folder: string; sibs: FileNode[]; onCurrent?: (n: FileNode) => void; close?: () => void; win?: WinEntry }>();

const host = ref<HTMLElement | null>(null);
const cur = ref(props.node);
const play = ref(false);
const info = ref('');
const stage = ref<{ keys?: KeyHandler } | null>(null);

function meta() {
  const el = footStartOf(host.value);
  if (el) el.textContent = [info.value, fmtSize(cur.value.size), t('viewer.changed', { time: fmtTime(cur.value.time) })].filter(Boolean).join(', ');
}
function show(n: FileNode, autoplay = false) {
  cur.value = n;
  play.value = autoplay;
  info.value = '';
  props.onCurrent?.(n);
  if (props.win) props.win.title = n.name;
  meta();
}
const onInfo = (text: string) => { info.value = text; meta(); };

let unbind = () => {};
onMounted(() => {
  meta();
  unbind = winKeys(() => host.value?.closest('.glim-modal-backdrop') as HTMLElement | null, (e, combo) => stage.value?.keys?.(e, combo));
  // The frame focuses its last button; the stage takes the keys for the gallery and the players instead.
  setTimeout(() => host.value?.focus({ preventScroll: true }), 30);
});
onBeforeUnmount(() => unbind());
defineExpose({ show: (n: FileNode) => nextTick(() => show(n)) });
</script>

<template>
  <div ref="host" class="cv-host" tabindex="-1">
    <div class="cv-box">
      <Stage ref="stage" :key="cur.name" :n="cur" :folder="folder" :sibs="sibs" :autoplay="play" @info="onInfo" @go="show" />
    </div>
  </div>
</template>
