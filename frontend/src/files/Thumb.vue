<script setup lang="ts">
// What a file looks like on a card: the server's thumbnail for pictures, films and documents, the first lines of
// text files, and a sketched page for documents, sheets and slides the server has no picture of.
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { G, glyphOf } from '@/core/glyphs';
import { fsText, thumbUrl } from '@/features/viewer/content';
import { ext, formatOf, hash, joinPath } from '@/features/viewer/kinds';

const props = defineProps<{ item: { name: string; dir?: boolean; time: number; size: number }; folder: string; duration?: string }>();

const TEXT_LIKE = ['text', 'code', 'config', 'log', 'subtitle', 'playlist', 'checksum', 'key', 'calendar', 'mail', 'weblink'];
const PICTURED = ['image', 'raw', 'video', 'pdf', 'doc', 'sheet', 'slides', 'ebook', 'audio'];

const f = computed(() => (props.item.dir ? '' : formatOf(props.item.name)));
const path = computed(() => joinPath(props.folder, props.item.name));
const hue = computed(() => hash(props.item.name) % 360);
const stemmed = computed(() => props.item.name.replace(/\.[^.]+$/, '').replace(/[-_.]+/g, ' '));
const tag = computed(() => (f.value === 'pdf' ? 'PDF' : ext(props.item.name).toUpperCase()));
const bars = computed(() => Array.from({ length: 18 }, (_, i) => 20 + (hash(props.item.name + i) % 80)));
const pic = ref(PICTURED.includes(f.value));
const lines = ref<string | null>(null);
const el = ref<HTMLElement | null>(null);

// Text is read only for cards that come into sight, so a long folder does not ask for every file at once.
let seen: IntersectionObserver | null = null;
onMounted(() => {
  if (!TEXT_LIKE.includes(f.value) || !el.value) return;
  seen = new IntersectionObserver((hits) => {
    if (!hits.some((h) => h.isIntersecting)) return;
    seen?.disconnect();
    fsText(path.value, 2048).then((t) => { lines.value = t.text.split('\n').slice(0, 16).join('\n'); }, () => { lines.value = ''; });
  });
  seen.observe(el.value);
});
onBeforeUnmount(() => seen?.disconnect());
</script>

<template>
  <span v-if="item.dir" class="thumb"><G :n="glyphOf(item)" /></span>
  <span v-else-if="pic && (f === 'image' || f === 'raw' || f === 'video')" class="thumb pic">
    <img class="scene" :src="thumbUrl(path, 'small', item.time)" alt="" loading="lazy" @error="pic = false">
    <template v-if="f === 'video'"><span class="play"><G n="IconPlay" /></span><span v-if="duration" class="badge glim-num">{{ duration }}</span></template>
  </span>
  <span v-else-if="pic && ['pdf', 'doc', 'sheet', 'slides', 'ebook'].includes(f)" class="thumb paper">
    <img class="page-pic" :src="thumbUrl(path, 'small', item.time)" alt="" loading="lazy" @error="pic = false"><span class="tag" :class="f">{{ tag }}</span>
  </span>
  <span v-else-if="f === 'audio'" class="thumb cover" :style="{ '--h': hue }">
    <img v-if="pic" class="scene" :src="thumbUrl(path, 'small', item.time)" alt="" loading="lazy" @error="pic = false">
    <template v-else><G n="IconFileAudio" /><span class="wave"><i v-for="(h, i) in bars" :key="i" :style="{ height: h + '%' }"></i></span></template>
  </span>
  <span v-else-if="TEXT_LIKE.includes(f)" ref="el" class="thumb paper"><span class="page code">{{ lines ?? '' }}</span></span>
  <span v-else-if="f === 'pdf' || f === 'doc'" class="thumb paper"><span class="page"><b>{{ stemmed }}</b><i v-for="i in 9" :key="i" :class="{ s: i === 4 || i === 9 }"></i></span><span class="tag" :class="f">{{ tag }}</span></span>
  <span v-else-if="f === 'sheet'" class="thumb paper"><span class="page grid"></span><span class="tag sheet">{{ tag }}</span></span>
  <span v-else-if="f === 'slides'" class="thumb paper"><span class="slide"><b>{{ stemmed }}</b><span class="bars"><i style="height:45%"></i><i style="height:80%"></i><i style="height:60%"></i></span></span><span class="tag slides">{{ tag }}</span></span>
  <span v-else-if="f === 'ebook'" class="thumb paper"><span class="book" :style="{ '--h': hue }"><b>{{ stemmed }}</b></span></span>
  <span v-else-if="f === 'font'" class="thumb font"><span>Aa</span><small>{{ stemmed }}</small></span>
  <span v-else class="thumb"><G :n="glyphOf(item)" /></span>
</template>

<style>
.thumb > img.scene { width: 100%; height: 100%; object-fit: cover; display: block; }
.thumb.paper > img.page-pic { height: 88%; max-width: 88%; object-fit: contain; border-radius: 2px; box-shadow: 0 1px 6px rgb(0 0 0 / .35); background: #f6f5f2; }
.thumb.cover > img.scene { position: absolute; inset: 0; }
</style>
