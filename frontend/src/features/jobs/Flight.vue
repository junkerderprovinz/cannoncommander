<script setup lang="ts">
// Where a job goes: its two ends, with the files flying from one to the other, each with the glyph of its type.
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { G, glyphOf } from '@/core/glyphs';
import { split } from '@/files/paths';

const props = defineProps<{ from: string; to: string; toGlyph?: string; file?: string; moving?: boolean; paused?: boolean }>();

const glyphFor = (path?: string) => (path ? glyphOf({ name: split(path).pop() || path }) : 'IconFile');
const resting = computed(() => glyphFor(props.file));
const flights = ref<{ id: number; g: string }[]>([]);
let next = 0;
let timer: ReturnType<typeof setTimeout> | undefined;

// The motion level sets the pace, so a calmer level sends fewer files, and none fly where motion is off.
function gap() {
  const root = document.documentElement;
  if (root.dataset.motion === 'off' || matchMedia('(prefers-reduced-motion: reduce)').matches) return 0;
  return parseFloat(getComputedStyle(root).getPropertyValue('--motion-fly-gap')) || 0;
}
function launch() {
  const s = gap();
  if (!s) flights.value = [];
  else if (props.moving && flights.value.length < 3) flights.value.push({ id: next++, g: glyphFor(props.file) });
  timer = setTimeout(launch, (s || 1) * 1000);
}
const land = (id: number) => { flights.value = flights.value.filter((f) => f.id !== id); };
watch(() => props.moving, (on) => {
  clearTimeout(timer);
  if (on) launch();
}, { immediate: true });
onBeforeUnmount(() => clearTimeout(timer));
</script>

<template>
  <div class="jfly" :class="{ paused, flying: flights.length > 0 }">
    <div class="jfly-end"><span class="jfly-tile"><G n="IconFolder" /></span><span class="jfly-name">{{ from }}</span></div>
    <div class="jfly-sky" aria-hidden="true">
      <span class="jfly-rest"><G :n="resting" /></span>
      <span v-for="f in flights" :key="f.id" class="jfly-lane" @animationend.self="land(f.id)"><span class="jfly-file"><G :n="f.g" /></span></span>
    </div>
    <div class="jfly-end jfly-end--to"><span class="jfly-name">{{ to }}</span><span class="jfly-tile"><G :n="toGlyph ?? 'IconFolderOpen'" /></span></div>
  </div>
</template>
