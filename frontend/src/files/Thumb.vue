<script setup lang="ts">
// A card shows what a file looks like: the server's picture for photos and films, the first lines of text files, and
// a sketched page for documents, sheets and slides. A feature can hand in a picture of its own first.
import { computed } from 'vue';
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
// The same hash as the prototype's, so a cover keeps its colour and its wave.
const hashOf = (s: string) => { let h = 2166136261; for (const c of s) h = Math.imul(h ^ c.charCodeAt(0), 16777619); return h >>> 0; };

const path = computed(() => join(props.folder, props.k.name));
const own = computed(() => extend.thumbProviders.map((fn) => fn(props.k, props.folder)).find((x) => x != null));
const f = computed(() => FORMAT_ID[ext(props.k.name)] || '');
const hue = computed(() => hashOf(props.k.name) % 360);
const stem = computed(() => props.k.name.replace(/\.[^.]+$/, '').replace(/[-_.]+/g, ' '));
const wave = computed(() => Array.from({ length: 18 }, (_, i) => 20 + (hashOf(props.k.name + i) % 80)));
const lines = (n: number, short: number[]) => Array.from({ length: n }, (_, i) => short.includes(i));
const text = computed(() => (TEXT_LIKE.includes(f.value) ? (listing.text(path.value) ?? '').split('\n').slice(0, 16).join('\n') : ''));
</script>

<template>
  <span v-if="own != null" class="thumb"><Render :node="own" /></span>
  <span v-else-if="k.dir" class="thumb"><G :n="glyphOf(k)" /></span>
  <span v-else-if="f === 'image' || f === 'raw'" class="thumb pic"><img class="scene" :src="api.thumbUrl(path)" loading="lazy" alt=""></span>
  <span v-else-if="f === 'video'" class="thumb pic"><img class="scene" :src="api.thumbUrl(path)" loading="lazy" alt=""><span class="play"><G n="IconPlay" /></span><span v-if="k.meta?.duration != null" class="badge glim-num">{{ COL.duration!.value(k, folder) }}</span></span>
  <span v-else-if="f === 'audio'" class="thumb cover" :style="{ '--h': hue }"><G n="IconFileAudio" /><span class="wave"><i v-for="(hgt, n) in wave" :key="n" :style="{ height: hgt + '%' }"></i></span></span>
  <span v-else-if="TEXT_LIKE.includes(f)" class="thumb paper"><span class="page code">{{ text }}</span></span>
  <span v-else-if="f === 'pdf' || f === 'doc'" class="thumb paper"><span class="page"><b>{{ stem }}</b><i v-for="(s, n) in lines(9, [3, 8])" :key="n" :class="{ s }"></i></span><span class="tag" :class="f">{{ f === 'pdf' ? 'PDF' : ext(k.name).toUpperCase() }}</span></span>
  <span v-else-if="f === 'sheet'" class="thumb paper"><span class="page grid"></span><span class="tag sheet">{{ ext(k.name).toUpperCase() }}</span></span>
  <span v-else-if="f === 'slides'" class="thumb paper"><span class="slide"><b>{{ stem }}</b><span class="bars"><i style="height:45%"></i><i style="height:80%"></i><i style="height:60%"></i></span></span><span class="tag slides">{{ ext(k.name).toUpperCase() }}</span></span>
  <span v-else-if="f === 'ebook'" class="thumb paper"><span class="book" :style="{ '--h': hue }"><b>{{ stem }}</b></span></span>
  <span v-else-if="f === 'font'" class="thumb font"><span>Aa</span><small>{{ stem }}</small></span>
  <span v-else class="thumb"><G :n="glyphOf(k)" /></span>
</template>
