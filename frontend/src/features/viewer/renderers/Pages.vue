<script setup lang="ts">
// Pages of a PDF or an office file as the server draws them, with page pictures to the left as in a PDF reader.
import { IconButton, Pill } from '@glimstone/vue';
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { type FirstPage, fsFirstPage, pageUrl } from '../content';
import { joinPath, plural } from '../kinds';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits & { (e: 'failed'): void }>();

const PAGE_W = 1536, THUMB_W = 240;
const path = computed(() => joinPath(props.folder, props.n.name));
const first = ref<FirstPage | null>(null);
const cur = ref(0);
const box = ref<HTMLElement | null>(null);
const count = computed(() => first.value?.count ?? 0);
const ar = computed(() => (first.value?.width ? `${first.value.width} / ${first.value.height}` : '1 / 1.414'));
const src = (k: number, w: number) => (k === 0 && w === PAGE_W && first.value ? first.value.url : pageUrl(path.value, k + 1, w, props.n.time));

onMounted(async () => {
  try {
    first.value = await fsFirstPage(path.value, props.compact ? 720 : PAGE_W, props.n.time);
    emit('info', plural(first.value.count, 'pages'));
  } catch {
    emit('failed');
  }
});
onBeforeUnmount(() => { if (first.value) URL.revokeObjectURL(first.value.url); });

function mark(k: number) {
  cur.value = k;
  nextTick(() => box.value?.parentElement?.querySelector('.cv-pthumb.cur')?.scrollIntoView({ block: 'nearest' }));
}
function go(k: number) {
  k = Math.max(0, Math.min(count.value - 1, k));
  box.value?.querySelector(`[data-page="${k}"]`)?.scrollIntoView({ block: 'start', behavior: 'smooth' });
  mark(k);
}
function scrolled() {
  const b = box.value!;
  const mid = b.getBoundingClientRect().top + b.clientHeight * 0.35;
  const k = [...b.querySelectorAll('[data-page]')].findLastIndex((p) => p.getBoundingClientRect().top <= mid);
  if (k >= 0 && k !== cur.value) mark(k);
}

defineExpose({
  keys: (_e: KeyboardEvent, combo: string) => {
    if (props.compact || !count.value) return;
    if (['PageDown', 'ArrowRight'].includes(combo)) return go(cur.value + 1), true;
    if (['PageUp', 'ArrowLeft'].includes(combo)) return go(cur.value - 1), true;
    if (combo === 'Home') return go(0), true;
    if (combo === 'End') return go(count.value - 1), true;
    return undefined;
  },
});
</script>

<template>
  <div v-if="compact" class="cv-paper img" :style="{ aspectRatio: ar }"><img v-if="first" :src="first.url" alt=""></div>
  <template v-else>
    <div class="cv-bar">
      <IconButton :tip="t('viewer.pages.prevTip')" :glyph="glyphComponent('IconBack')" data-pg="-1" @click="go(cur - 1)" />
      <Pill id="cv-pgn" class="glim-num">{{ t('viewer.pages.of', { i: cur + 1, n: count || 1 }) }}</Pill>
      <IconButton :tip="t('viewer.pages.nextTip')" :glyph="glyphComponent('IconForward')" data-pg="1" @click="go(cur + 1)" />
    </div>
    <div class="cv-doc">
      <div v-if="count > 1" class="cv-pthumbs">
        <button v-for="k in count" :key="k" class="cv-pthumb" :class="{ cur: k - 1 === cur }" :data-pk="k - 1" :aria-label="t('viewer.pages.page', { n: k })" @click="go(k - 1)">
          <span class="cv-paper img" :style="{ aspectRatio: ar }"><img :src="src(k - 1, THUMB_W)" alt="" loading="lazy"></span><span class="glim-num">{{ k }}</span>
        </button>
      </div>
      <div ref="box" class="cv-pages" @scroll="scrolled">
        <div v-for="k in count" :key="k" class="cv-paper img" :data-page="k - 1" :style="{ aspectRatio: ar }"><img :src="src(k - 1, PAGE_W)" alt="" loading="lazy"></div>
      </div>
    </div>
  </template>
</template>
