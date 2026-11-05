<script setup lang="ts">
// The slides of a presentation: the preview picture Office keeps in the file stands for the first, the text of
// each slide for the rest.
import { IconButton, Pill } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { rawUrl } from '../content';
import { joinPath, plural } from '../kinds';
import { readPptx, type Slide } from '../ooxml';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits & { (e: 'failed'): void }>();

const slides = ref<Slide[]>([]);
const picture = ref('');
const cur = ref(0);
const count = computed(() => slides.value.length);

onMounted(async () => {
  try {
    const res = await fetch(rawUrl(joinPath(props.folder, props.n.name)));
    if (!res.ok) throw new Error(String(res.status));
    const got = readPptx(new Uint8Array(await res.arrayBuffer()));
    slides.value = got.slides.length ? got.slides : [{ title: '', lines: [] }];
    if (got.thumbnail) picture.value = URL.createObjectURL(new Blob([got.thumbnail as BlobPart], { type: 'image/jpeg' }));
    emit('info', plural(slides.value.length, 'slides'));
  } catch {
    emit('failed');
  }
});
onBeforeUnmount(() => { if (picture.value) URL.revokeObjectURL(picture.value); });

const go = (k: number) => { cur.value = Math.max(0, Math.min(count.value - 1, k)); };
defineExpose({
  keys: (_e: KeyboardEvent, combo: string) => {
    if (props.compact) return;
    if (['ArrowRight', 'PageDown', ' '].includes(combo)) return go(cur.value + 1), true;
    if (['ArrowLeft', 'PageUp'].includes(combo)) return go(cur.value - 1), true;
    return undefined;
  },
});
</script>

<template>
  <template v-if="count">
    <div v-if="compact" class="cv-slide"><img v-if="picture" :src="picture" alt=""><div v-else class="sl"><h2>{{ slides[0]!.title }}</h2></div></div>
    <template v-else>
      <div class="cv-bar">
        <IconButton :tip="t('viewer.slides.prev')" :glyph="glyphComponent('IconBack')" data-sl="-1" @click="go(cur - 1)" />
        <Pill class="glim-num">{{ t('viewer.slides.of', { i: cur + 1, n: count }) }}</Pill>
        <IconButton :tip="t('viewer.slides.next')" :glyph="glyphComponent('IconForward')" data-sl="1" @click="go(cur + 1)" />
      </div>
      <div class="cv-doc">
        <div v-if="count > 1" class="cv-pthumbs">
          <button v-for="(s, k) in slides" :key="k" class="cv-pthumb" :class="{ cur: k === cur }" :data-sk="k" :aria-label="t('viewer.slides.slide', { n: k + 1 })" @click="go(k)">
            <span class="cv-slide"><img v-if="k === 0 && picture" :src="picture" alt=""><span v-else class="sl"><h2>{{ s.title }}</h2><ul><li v-for="(l, i) in s.lines" :key="i">{{ l }}</li></ul></span></span>
            <span class="glim-num">{{ k + 1 }}</span>
          </button>
        </div>
        <div class="cv-stagebox">
          <div class="cv-slide big">
            <img v-if="cur === 0 && picture" :src="picture" alt="">
            <div v-else class="sl"><h2>{{ slides[cur]!.title }}</h2><ul><li v-for="(l, i) in slides[cur]!.lines" :key="i">{{ l }}</li></ul></div>
          </div>
        </div>
      </div>
    </template>
  </template>
</template>
