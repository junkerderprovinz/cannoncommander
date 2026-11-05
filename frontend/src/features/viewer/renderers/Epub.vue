<script setup lang="ts">
// An e-book page by page on paper, in the reader's own typeface; the arrows turn the pages.
import { IconButton, Pill } from '@glimstone/vue';
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { rawUrl } from '../content';
import { joinPath } from '../kinds';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits & { (e: 'failed'): void }>();
const host = ref<HTMLElement | null>(null);
const where = ref('');
type Rendition = import('@likecoin/epub-ts').Rendition;
type Book = import('@likecoin/epub-ts').Book;
let book: Book | null = null;
let rendition: Rendition | null = null;

onMounted(async () => {
  try {
    const res = await fetch(rawUrl(joinPath(props.folder, props.n.name)));
    if (!res.ok) throw new Error(String(res.status));
    const { default: ePub } = await import('@likecoin/epub-ts');
    book = ePub(await res.arrayBuffer());
    rendition = book.renderTo(host.value!, { width: '100%', height: '100%', flow: 'paginated', spread: 'none' });
    rendition.themes.default({ body: { 'font-family': '"Noto Serif", Georgia, serif', color: '#1d1d1f', background: '#fff' } });
    rendition.on('relocated', (loc: { start?: { displayed?: { page: number; total: number } } }) => {
      const d = loc.start?.displayed;
      where.value = d ? t('viewer.pages.of', { i: d.page, n: d.total }) : '';
    });
    await rendition.display();
    const meta = await book.loaded.metadata;
    emit('info', [t('viewer.epub.book'), meta?.creator].filter(Boolean).join(', '));
  } catch {
    emit('failed');
  }
});
onBeforeUnmount(() => book?.destroy());

const next = () => rendition?.next();
const prev = () => rendition?.prev();
defineExpose({
  keys: (_e: KeyboardEvent, combo: string) => {
    if (props.compact) return;
    if (['ArrowRight', 'PageDown', ' '].includes(combo)) return next(), true;
    if (['ArrowLeft', 'PageUp'].includes(combo)) return prev(), true;
    return undefined;
  },
});
</script>

<template>
  <div v-if="!compact" class="cv-bar">
    <IconButton :tip="t('viewer.epub.prev')" :glyph="glyphComponent('IconBack')" @click="prev" />
    <Pill v-if="where" class="glim-num">{{ where }}</Pill>
    <IconButton :tip="t('viewer.epub.next')" :glyph="glyphComponent('IconForward')" @click="next" />
  </div>
  <div class="cv-book" :class="{ compact }"><div ref="host" class="cv-book-page"></div></div>
</template>
