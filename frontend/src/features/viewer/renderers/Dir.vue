<script setup lang="ts">
// A folder in the preview bar: its pictures when it is mostly pictures, else its first entries.
import { EmptyState, GlyphTile, Pill } from '@glimstone/vue';
import { computed, onMounted } from 'vue';
import { glyphOf, glyphComponent } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import { byName, t } from '@/i18n';
import SizeText from '@/files/SizeText.vue';
import { useListing } from '@/stores/listing';
import { thumbUrl } from '../content';
import { joinPath, plural, viewKind } from '../kinds';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits>();
const listing = useListing();

const path = computed(() => joinPath(props.folder, props.n.name));
const kids = computed(() => (listing.lists.get(path.value)?.items || []).filter((k) => prefs.showHidden || !k.name.startsWith('.')));
const pics = computed(() => kids.value.filter((k) => viewKind(k) === 'img'));
const top = computed(() => [...kids.value].sort((a, b) => Number(b.dir) - Number(a.dir) || byName(a.name, b.name)));

function info() {
  const dirs = kids.value.filter((k) => k.dir).length, files = kids.value.length - dirs;
  emit('info', kids.value.length ? [dirs ? plural(dirs, 'dirs') : '', files ? plural(files, 'files') : ''].filter(Boolean).join(', ') : t('viewer.dir.none'));
}
onMounted(async () => {
  await listing.load(path.value).catch(() => null);
  info();
});
</script>

<template>
  <EmptyState v-if="!kids.length" nested :glyph="glyphComponent('IconFolder')" :title="t('viewer.dir.empty')" />
  <template v-else-if="pics.length >= kids.length / 2">
    <div class="cv-grid"><div v-for="k in pics.slice(0, 12)" :key="k.name" class="cv-cell"><img :src="thumbUrl(joinPath(path, k.name), 'small', k.time)" alt="" loading="lazy"></div></div>
    <div v-if="pics.length > 12" class="muted cv-more">{{ t('viewer.and', { rest: plural(pics.length - 12, 'morePictures') }) }}</div>
  </template>
  <template v-else>
    <div class="cv-mini">
      <div v-for="k in top.slice(0, 12)" :key="k.name" class="cv-mrow"><GlyphTile :glyph="glyphComponent(glyphOf(k))" /><span class="t">{{ k.name }}</span><Pill class="glim-num"><SizeText :k="k" /></Pill></div>
    </div>
    <div v-if="top.length > 12" class="muted cv-more">{{ t('viewer.and', { rest: plural(top.length - 12, 'moreObjects') }) }}</div>
  </template>
</template>
