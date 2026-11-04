<script setup lang="ts">
// A card shows what a file looks like: the server's picture for photos, films and documents, the first lines of text
// files, and a sketched page for anything the server has no picture of. A feature can hand in a picture of its own first.
import { computed, ref, watch } from 'vue';
import { api } from '@/api/client';
import { extend } from '@/core/extend';
import { ext, FORMAT_ID, G, glyphOf } from '@/core/glyphs';
import { Render } from '@/core/Render';
import { useListing } from '@/stores/listing';
import { COL } from './columns';
import type { Entry } from './entries';
import { join } from './paths';

const props = defineProps<{ k: Entry; folder: string }>();
const listing = useListing();

const TEXT_LIKE = ['text', 'code', 'config', 'log', 'subtitle', 'playlist', 'checksum', 'key', 'calendar', 'mail', 'weblink'];
const PAGES = ['pdf', 'doc', 'sheet', 'slides', 'ebook'];
// The same hash as the prototype's, so a cover keeps its colour and its wave.
const hashOf = (s: string) => { let h = 2166136261; for (const c of s) h = Math.imul(h ^ c.charCodeAt(0), 16777619); return h >>> 0; };

const path = computed(() => join(props.folder, props.k.name));
const own = computed(() => extend.thumbProviders.map((fn) => fn(props.k, props.folder)).find((x) => x != null));
const f = computed(() => FORMAT_ID[ext(props.k.name)] || '');
const hue = computed(() => hashOf(props.k.name) % 360);
const stem = computed(() => props.k.name.replace(/\.[^.]+$/, '').replace(/[-_.]+/g, ' '));
const tag = computed(() => (f.value === 'pdf' ? 'PDF' : ext(props.k.name).toUpperCase()));
const wave = computed(() => Array.from({ length: 18 }, (_, i) => 20 + (hashOf(props.k.name + i) % 80)));
const lines = (n: number, short: number[]) => Array.from({ length: n }, (_, i) => short.includes(i));
const text = computed(() => (TEXT_LIKE.includes(f.value) ? (listing.text(path.value) ?? '').split('\n').slice(0, 16).join('\n') : ''));
const src = computed(() => api.thumbUrl(path.value, 'small', props.k.time));
// A file the server cannot picture falls back to the sketch, and a changed file gets another try.
const pic = ref(true);
watch(src, () => { pic.value = true; });
</script>

<template>
  <span v-if="own != null" class="thumb"><Render :node="own" /></span>
  <span v-else-if="k.dir && k.picture && pic" class="thumb pic folderpic"><img class="scene" :src="src" loading="lazy" alt="" @error="pic = false"><span class="fbadge"><G n="IconFolder" /></span></span>
  <span v-else-if="k.dir" class="thumb"><G :n="glyphOf(k)" /></span>
  <span v-else-if="pic && (f === 'image' || f === 'raw' || f === 'video')" class="thumb pic">
    <img class="scene" :src="src" loading="lazy" alt="" @error="pic = false">
    <template v-if="f === 'video'"><span class="play"><G n="IconPlay" /></span><span v-if="k.meta?.duration != null" class="badge glim-num">{{ COL.duration!.value(k, folder) }}</span></template>
  </span>
  <span v-else-if="pic && PAGES.includes(f)" class="thumb paper"><img class="page-pic" :src="src" loading="lazy" alt="" @error="pic = false"><span class="tag" :class="f">{{ tag }}</span></span>
  <span v-else-if="f === 'audio'" class="thumb cover" :style="{ '--h': hue }">
    <img v-if="pic" class="scene" :src="src" loading="lazy" alt="" @error="pic = false">
    <template v-else><G n="IconFileAudio" /><span class="wave"><i v-for="(hgt, n) in wave" :key="n" :style="{ height: hgt + '%' }"></i></span></template>
  </span>
  <span v-else-if="TEXT_LIKE.includes(f)" class="thumb paper"><span class="page code">{{ text }}</span></span>
  <span v-else-if="f === 'pdf' || f === 'doc'" class="thumb paper"><span class="page"><b>{{ stem }}</b><i v-for="(s, n) in lines(9, [3, 8])" :key="n" :class="{ s }"></i></span><span class="tag" :class="f">{{ tag }}</span></span>
  <span v-else-if="f === 'sheet'" class="thumb paper"><span class="page grid"></span><span class="tag sheet">{{ tag }}</span></span>
  <span v-else-if="f === 'slides'" class="thumb paper"><span class="slide"><b>{{ stem }}</b><span class="bars"><i style="height:45%"></i><i style="height:80%"></i><i style="height:60%"></i></span></span><span class="tag slides">{{ tag }}</span></span>
  <span v-else-if="f === 'ebook'" class="thumb paper"><span class="book" :style="{ '--h': hue }"><b>{{ stem }}</b></span></span>
  <span v-else-if="f === 'font'" class="thumb font"><span>Aa</span><small>{{ stem }}</small></span>
  <span v-else class="thumb"><G :n="glyphOf(k)" /></span>
</template>

<style>
.thumb > img.scene { width: 100%; height: 100%; object-fit: cover; display: block; }
.thumb.paper > img.page-pic { height: 88%; max-width: 88%; object-fit: contain; border-radius: 2px; box-shadow: 0 1px 6px rgb(0 0 0 / .35); background: #f6f5f2; }
.thumb.cover > img.scene { position: absolute; inset: 0; }
.thumb.folderpic .fbadge { position: absolute; left: .4rem; bottom: .4rem; width: 1.6rem; height: 1.6rem; border-radius: var(--radius-control); display: grid; place-items: center; background: color-mix(in srgb, var(--carbon-surface) 82%, transparent); box-shadow: var(--hairline); }
.thumb.folderpic .fbadge svg { width: .9rem; height: .9rem; color: var(--accent-glyph); }
</style>
