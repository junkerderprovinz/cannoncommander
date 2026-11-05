<script lang="ts">
import { segItems } from '@/ui/kit';
import { Pill, Selector } from '@glimstone/vue';
import { ref } from 'vue';

// The zoom carries over from one picture to the next, as it does in the prototype.
const zoom = ref('fit');
</script>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref as vref, watch } from 'vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { rawUrl, thumbUrl } from '../content';
import { browserDraws, joinPath } from '../kinds';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits>();

const path = computed(() => joinPath(props.folder, props.n.name));
const src = computed(() => (browserDraws(props.n.name) ? rawUrl(path.value) : thumbUrl(path.value, 'large', props.n.time)));
const dims = vref<[number, number] | null>(null);
const i = computed(() => props.sibs.findIndex((s) => s.name === props.n.name));
const many = computed(() => !props.compact && props.sibs.length > 1);
const stage = vref<HTMLElement | null>(null);

function loaded(e: Event) {
  const img = e.target as HTMLImageElement;
  if (!img.naturalWidth) return;
  dims.value = [img.naturalWidth, img.naturalHeight];
  emit('info', `${img.naturalWidth} × ${img.naturalHeight}`);
  if (zoom.value === 'full') nextTick(center);
}

const picStyle = computed(() => {
  const [w, h] = dims.value || [0, 0];
  return zoom.value === 'full' && w ? { width: `${w}px`, height: `${h}px` } : { '--ar': `${w || 4} / ${h || 3}` };
});

const step = (d: number) => emit('go', props.sibs[(i.value + d + props.sibs.length) % props.sibs.length]!);

function center() {
  const s = stage.value;
  if (!s) return;
  s.scrollLeft = (s.scrollWidth - s.clientWidth) / 2;
  s.scrollTop = (s.scrollHeight - s.clientHeight) / 2;
}
watch(zoom, () => nextTick(center));

// At 100 % the picture is larger than the stage, so dragging moves it like a hand on paper.
let pan: { x: number; y: number; l: number; t: number } | null = null;
const panning = vref(false);
function down(e: MouseEvent) {
  if (zoom.value !== 'full' || (e.target as Element).closest('button')) return;
  pan = { x: e.clientX, y: e.clientY, l: stage.value!.scrollLeft, t: stage.value!.scrollTop };
  panning.value = true;
}
function move(e: MouseEvent) {
  if (!pan) return;
  stage.value!.scrollLeft = pan.l - (e.clientX - pan.x);
  stage.value!.scrollTop = pan.t - (e.clientY - pan.y);
}
const up = () => { pan = null; panning.value = false; };

onMounted(() => document.querySelector('.cv-thumb.cur')?.scrollIntoView({ block: 'nearest', inline: 'center' }));

defineExpose({
  keys: (_e: KeyboardEvent, combo: string) => {
    if (props.compact) return;
    if (combo === 'ArrowRight' || combo === 'PageDown') return step(1), true;
    if (combo === 'ArrowLeft' || combo === 'PageUp') return step(-1), true;
    if (combo === 'Home') return emit('go', props.sibs[0]!), true;
    if (combo === 'End') return emit('go', props.sibs.at(-1)!), true;
    return undefined;
  },
});
</script>

<template>
  <div v-if="compact" class="cv-pic-c"><img :src="src" alt="" @load="loaded"></div>
  <template v-else>
    <div class="cv-bar">
      <Pill v-if="many" class="glim-num">{{ t('viewer.img.of', { i: i + 1, n: sibs.length }) }}</Pill><span class="grow"></span>
      <Selector id="cv-zoom" v-model="zoom" :items="segItems([['fit', t('viewer.img.fit'), 'IconCompact'], ['full', '100 %', 'IconSearch']])" size="sm" activation="manual" inline button-height />
    </div>
    <div id="cv-stage" ref="stage" class="cv-stage" :class="[zoom, { panning }]" @mousedown="down" @mousemove="move" @mouseup="up" @mouseleave="up">
      <div class="cv-pic" :style="picStyle"><img :src="src" alt="" draggable="false" @load="loaded"></div>
      <template v-if="many">
        <button class="cv-nav prev" data-step="-1" :data-tip="t('viewer.img.prevTip')" :aria-label="t('viewer.img.prev')" @click="step(-1)"><G n="IconBack" /></button>
        <button class="cv-nav next" data-step="1" :data-tip="t('viewer.img.nextTip')" :aria-label="t('viewer.img.next')" @click="step(1)"><G n="IconForward" /></button>
      </template>
    </div>
    <div v-if="many" class="cv-strip">
      <button v-for="(s, k) in sibs" :key="s.name" class="cv-thumb" :class="{ cur: s.name === n.name }" :data-k="k" :aria-label="s.name" @click="emit('go', s)">
        <img :src="thumbUrl(joinPath(folder, s.name), 'small', s.time)" alt="" loading="lazy">
      </button>
    </div>
  </template>
</template>
