<script setup lang="ts">
// A file of the link over the page: pictures, video, audio, PDF and text the way the browser shows them. Arrows
// step through the files of the folder, Escape closes.
import { IconButton } from '@glimstone/vue';
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import type { PublicItem } from '@/api/share';
import { fmtSize } from '@/core/format';
import { G, glyphOf, glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { previewKind } from './preview';

const props = defineProps<{ item: PublicItem; src: string; download: string; text: () => Promise<string>; many: boolean }>();
const emit = defineEmits<{ close: []; step: [by: number] }>();
const s = (key: string) => t('share.' + key);

const words = ref<string | null>(null);
const box = ref<HTMLElement | null>(null);
async function read() {
  words.value = null;
  const k = previewKind(props.item.name);
  if (k !== 'text' && k !== 'code') return;
  try { words.value = await props.text(); } catch { words.value = ''; }
}
watch(() => props.item, read);

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close');
  else if (props.many && e.key === 'ArrowLeft') emit('step', -1);
  else if (props.many && e.key === 'ArrowRight') emit('step', 1);
  else return;
  e.preventDefault();
}
onMounted(() => {
  read();
  addEventListener('keydown', onKey);
  box.value?.focus();
});
onBeforeUnmount(() => removeEventListener('keydown', onKey));
</script>

<template>
  <div class="sp-scrim" @click.self="emit('close')">
    <div ref="box" class="sp-view glim-card" role="dialog" :aria-label="item.name" tabindex="-1">
      <div class="sp-vhead">
        <G :n="glyphOf(item)" />
        <span class="sp-vname">{{ item.name }}</span>
        <span class="muted glim-num">{{ fmtSize(item.size) }}</span>
        <IconButton v-if="many" :tip="s('prev')" :glyph="glyphComponent('IconBack')" @click="emit('step', -1)" />
        <IconButton v-if="many" :tip="s('next')" :glyph="glyphComponent('IconForward')" @click="emit('step', 1)" />
        <a class="glim-btn-bare glim-icon-btn" :href="download" :data-tip="s('download')" :aria-label="s('download')"><G n="IconDownload" /></a>
        <IconButton :tip="s('close')" :glyph="glyphComponent('IconClose')" data-sp="close" @click="emit('close')" />
      </div>
      <div class="sp-vbody" :class="previewKind(item.name)">
        <img v-if="previewKind(item.name) === 'img'" :src="src" :alt="item.name">
        <video v-else-if="previewKind(item.name) === 'video'" :src="src" controls preload="metadata"></video>
        <audio v-else-if="previewKind(item.name) === 'audio'" :src="src" controls></audio>
        <iframe v-else-if="previewKind(item.name) === 'pdf'" :src="src" :title="item.name"></iframe>
        <pre v-else-if="words !== null" class="mono">{{ words }}</pre>
        <div v-else class="empty"><G :n="glyphOf(item)" /><div>{{ s('noPreview') }}</div></div>
      </div>
    </div>
  </div>
</template>
