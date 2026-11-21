<script setup lang="ts">
// Prototype: where a job goes, two ends with the files flying from one to the other. Each file shows the glyph of its
// type, taken from the file the job is at.
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { G, glyphOf } from '@/core/glyphs';

const props = defineProps<{ from: string; to: string; toGlyph?: string; file: string; moving?: boolean; paused?: boolean; compact?: boolean }>();

const glyphFor = (name: string) => glyphOf({ name: name.replace(/\/$/, ''), dir: name.endsWith('/') });
const resting = computed(() => glyphFor(props.file));
const flights = ref<{ id: number; g: string }[]>([]);
let next = 0;
let timer: ReturnType<typeof setTimeout> | undefined;

// The pace comes from the motion level, so a calmer level sends fewer files, and none fly where motion is off.
function pace() {
  const root = document.documentElement;
  if (root.dataset.motion === 'off' || matchMedia('(prefers-reduced-motion: reduce)').matches) return 0;
  return parseFloat(getComputedStyle(root).getPropertyValue('--motion-fly-gap')) || 0;
}
function launch() {
  const gap = pace();
  if (!gap) flights.value = [];
  if (gap && props.moving && flights.value.length < 3) flights.value.push({ id: next++, g: glyphFor(props.file) });
  timer = setTimeout(launch, (gap || 1) * 1000);
}
const land = (id: number) => { flights.value = flights.value.filter((f) => f.id !== id); };
watch(() => props.moving, (on) => {
  clearTimeout(timer);
  if (on) launch();
}, { immediate: true });
onBeforeUnmount(() => clearTimeout(timer));
</script>

<template>
  <div class="flow" :class="{ moving, paused, compact, flying: flights.length > 0 }">
    <div class="flow-end"><span class="flow-dot"><G n="IconFolder" /></span><span class="flow-name">{{ from }}</span></div>
    <div class="flow-sky" aria-hidden="true">
      <span class="flow-rest"><G :n="resting" /></span>
      <span v-for="f in flights" :key="f.id" class="flow-lane" @animationend.self="land(f.id)"><span class="flow-file"><G :n="f.g" /></span></span>
    </div>
    <div class="flow-end flow-end--to"><span class="flow-name">{{ to }}</span><span class="flow-dot"><G :n="toGlyph ?? 'IconFolderOpen'" /></span></div>
  </div>
</template>
