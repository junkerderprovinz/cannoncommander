<script lang="ts">
import { reactive } from 'vue';

// The pictures every row shares: the ones loaded, and the ones that failed or are on their way, so a folder of many
// files of one type asks once and no row waits for a slow one.
const loaded = reactive(new Set<string>());
const asked = new Set<string>();
const MAX_KEPT = 5000;

function load(src: string) {
  if (asked.has(src)) return;
  if (asked.size > MAX_KEPT) {
    asked.clear();
    loaded.clear();
  }
  asked.add(src);
  const img = new Image();
  img.onload = () => loaded.add(src);
  img.src = src;
}
</script>

<script setup lang="ts">
// An entry's glyph, or the picture a feature hands in for it, such as a program's own icon. The glyph stands until
// the picture has arrived, and stays when it never does.
import { computed, watchEffect } from 'vue';
import { extend } from '@/core/extend';
import { G, glyphOf } from '@/core/glyphs';
import type { Entry } from './entries';

const props = defineProps<{ k: Entry; folder: string; px: number }>();
const src = computed(() => extend.entryIcons.map((fn) => fn(props.k, props.folder, props.px)).find(Boolean) ?? '');
watchEffect(() => { if (src.value) load(src.value); });
</script>

<template>
  <img v-if="src && loaded.has(src)" class="entry-icon" :src="src" alt="" draggable="false">
  <G v-else :n="glyphOf(k)" />
</template>

<style>
img.entry-icon { object-fit: contain; display: block; }
.glyph > img.entry-icon { width: 1.05rem; height: 1.05rem; }
.view-compact .glyph > img.entry-icon { width: .9rem; height: .9rem; }
.row .thumb > img.entry-icon { width: 2.4rem; height: 2.4rem; }
.view-large .row .thumb > img.entry-icon { width: 3.4rem; height: 3.4rem; }
.list.zoomed .row .thumb > img.entry-icon { width: calc(var(--zoom-tile) * .265); height: calc(var(--zoom-tile) * .265); }
.citem .thumb > img.entry-icon { width: 1.5rem; height: 1.5rem; }
</style>
