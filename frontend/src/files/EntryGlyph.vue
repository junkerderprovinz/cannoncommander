<script setup lang="ts">
// An entry's glyph, or the picture a feature hands in for it, such as a program's own icon. A picture that does not
// load leaves the glyph in its place.
import { computed, ref } from 'vue';
import { extend } from '@/core/extend';
import { G, glyphOf } from '@/core/glyphs';
import type { Entry } from './entries';

const props = defineProps<{ k: Entry; folder: string; px: number }>();
const src = computed(() => extend.entryIcons.map((fn) => fn(props.k, props.folder, props.px)).find(Boolean) ?? '');
const broken = ref('');
</script>

<template>
  <img v-if="src && src !== broken" class="entry-icon" :src="src" alt="" draggable="false" loading="lazy" @error="broken = src">
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
