<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { rawUrl, thumbUrl } from '../content';
import { ext, fmtDur, hash, joinPath, rng, stem } from '../kinds';
import { esc } from '../markdown';
import { madeUpBars, realBars } from './peaks';
import { usePlayer } from './player';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits>();

const path = computed(() => joinPath(props.folder, props.n.name));
const artist = computed(() => props.folder.split('/').filter(Boolean).at(-1) || '');
const audio = ref<HTMLAudioElement | null>(null);
const { st, toggle, scrub, keys } = usePlayer(audio);
const i = computed(() => props.sibs.findIndex((s) => s.name === props.n.name));
const many = computed(() => props.sibs.length > 1);
const count = props.compact ? 160 : 260;
const wave = ref(madeUpBars(props.n.name, count));
const ownCover = ref(true);

// Without a cover in the file, a plain gradient in the folder's colour stands in for one.
const cover = computed(() => {
  const r = rng(props.folder), hue = Math.floor(r() * 360), id = 'cv' + hash(props.folder + props.n.name);
  const rings = Array.from({ length: 5 }, (_, k) => `<circle cx="${30 + r() * 40}" cy="${30 + r() * 40}" r="${8 + k * 9}" fill="none" stroke="#fff" stroke-opacity="${0.08 + r() * 0.12}" stroke-width="${1 + r() * 3}"/>`).join('');
  return `<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg"><defs><linearGradient id="${id}" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="hsl(${hue} 55% 55%)"/><stop offset="1" stop-color="hsl(${(hue + 60) % 360} 50% 22%)"/></linearGradient></defs>`
    + `<rect width="100" height="100" fill="url(#${id})"/>${rings}<text x="8" y="92" fill="#fff" fill-opacity=".85" font-family="Noto Sans, sans-serif" font-size="7" font-weight="700" letter-spacing=".6">${esc(artist.value.toUpperCase().slice(0, 22))}</text></svg>`;
});

const info = () => emit('info', [ext(props.n.name).toUpperCase(), st.dur ? fmtDur(st.dur) : ''].filter(Boolean).join(', '));
watch(() => st.dur, info);

function go(d: number | null, to?: number) {
  const k = to ?? (i.value + d! + props.sibs.length) % props.sibs.length;
  emit('go', props.sibs[k]!, st.playing);
}

const fraction = computed(() => (st.dur ? st.t / st.dur : 0));

onMounted(() => {
  info();
  realBars(path.value, props.n.size, count).then((b) => { if (b) wave.value = b; });
  if (props.autoplay) toggle();
});
defineExpose({ keys });
</script>

<template>
  <div class="cv-audio" :class="{ compact, playing: st.playing }">
    <audio ref="audio" :src="rawUrl(path)" preload="metadata"></audio>
    <div class="cv-cover">
      <img v-if="ownCover" :src="thumbUrl(path, 'large', n.time)" alt="" @error="ownCover = false">
      <span v-else v-html="cover"></span>
    </div>
    <div class="cv-atitle"><div class="t">{{ stem(n.name) }}</div><div class="muted">{{ artist }}</div></div>
    <div class="cv-scrub cv-wave" role="slider" :aria-label="t('viewer.media.position')" tabindex="0" @pointerdown="scrub">
      <i v-for="(h, k) in wave" :key="k" :class="{ on: k / wave.length < fraction }" :style="{ height: (h * 100).toFixed(0) + '%' }"></i>
    </div>
    <div class="cv-actl">
      <span class="glim-num cv-now">{{ fmtDur(st.t) }}</span>
      <span class="cv-tr">
        <button v-if="many && !compact" class="btn icon" data-step="-1" :data-tip="t('viewer.media.prevTrack')" :aria-label="t('viewer.media.prevTrack')" @click="go(-1)"><G n="IconFirst" /></button>
        <button class="btn icon cv-playbtn" data-play data-pi :data-tip="st.playing ? t('viewer.media.pauseTip') : t('viewer.media.playTip')" :aria-label="st.playing ? t('viewer.media.pause') : t('viewer.media.play')" @click="toggle">
          <G :n="st.playing ? 'IconPause' : 'IconPlay'" />
        </button>
        <button v-if="many && !compact" class="btn icon" data-step="1" :data-tip="t('viewer.media.nextTrack')" :aria-label="t('viewer.media.nextTrack')" @click="go(1)"><G n="IconLatest" /></button>
      </span>
      <span class="glim-num muted">{{ fmtDur(st.dur) }}</span>
    </div>
    <div v-if="many && !compact" class="well cv-tracks">
      <div class="eyebrow">{{ artist }}</div>
      <button v-for="(s, k) in sibs" :key="s.name" class="nav-row" :class="{ cur: s.name === n.name }" :data-k="k" @click="go(null, k)">
        <G n="IconFileAudio" /><span>{{ stem(s.name) }}</span><span class="meta glim-num">{{ s.name === n.name && st.dur ? fmtDur(st.dur) : '' }}</span>
      </button>
    </div>
  </div>
</template>
