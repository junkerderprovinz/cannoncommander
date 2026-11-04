<script lang="ts">
import { reactive } from 'vue';

// The chosen subtitles and sound track carry over to the next film.
const pick = reactive({ sub: 'off', audio: 0 });
</script>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { run } from '@/core/actions';
import { G } from '@/core/glyphs';
import { menu } from '@/core/menu';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { fsText, rawUrl, thumbUrl } from '../content';
import { filmOf, filmTitle, fmtDur, joinPath } from '../kinds';
import { usePlayer } from './player';
import { type Cue, cueAt, parseCues, subsFor } from './subs';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits>();
const listing = useListing();

const path = computed(() => joinPath(props.folder, props.n.name));
const film = computed(() => filmOf(props.n.name, props.folder));
const title = computed(() => filmTitle(film.value));
const poster = computed(() => thumbUrl(path.value, props.compact ? 'small' : 'large', props.n.time));
const video = ref<HTMLVideoElement | null>(null);
const { st, toggle, scrub, keys } = usePlayer(video);
const noPoster = ref(false);
const seen = computed(() => st.playing || st.t > 0);

const files = computed(() => subsFor(props.n.name, (listing.lists.get(props.folder)?.items || []).map((k) => k.name)));
// Tracks inside the file, where the browser shows them, come after the files beside it.
const inner = ref<TextTrack[]>([]);
const subs = computed<[string, string][]>(() => [
  ['off', t('viewer.media.off')],
  ...files.value.map((f): [string, string] => ['file:' + f.name, f.label]),
  ...inner.value.map((tr, k): [string, string] => ['track:' + k, tr.label || tr.language || t('viewer.media.track', { n: k + 1 })]),
]);
const subLabel = computed(() => subs.value.find((s) => s[0] === pick.sub)?.[1] ?? t('viewer.media.off'));
const cues = ref<Cue[]>([]);
const line = ref('');

interface AudioTrackLike { label: string; language: string; enabled: boolean }
const audios = ref<AudioTrackLike[]>([]);

watch(() => pick.sub, async (sub) => {
  cues.value = [];
  line.value = '';
  inner.value.forEach((tr, k) => { tr.mode = sub === 'track:' + k ? 'hidden' : 'disabled'; });
  if (sub.startsWith('file:')) {
    const text = await fsText(joinPath(props.folder, sub.slice(5))).catch(() => null);
    if (text && pick.sub === sub) cues.value = parseCues(text.text);
  }
});
watch(() => st.t, (s) => {
  if (pick.sub.startsWith('file:')) line.value = cueAt(cues.value, s);
});

function metadata() {
  const v = video.value!;
  emit('info', [v.videoWidth ? `${v.videoWidth} × ${v.videoHeight}` : '', st.dur ? fmtDur(st.dur) : ''].filter(Boolean).join(', '));
  inner.value = [...v.textTracks].filter((tr) => tr.kind === 'subtitles' || tr.kind === 'captions');
  inner.value.forEach((tr, k) => tr.addEventListener('cuechange', () => {
    if (pick.sub === 'track:' + k) line.value = [...(tr.activeCues || [])].map((c) => (c as VTTCue).text).join('\n');
  }));
  const at = (v as unknown as { audioTracks?: ArrayLike<AudioTrackLike> }).audioTracks;
  audios.value = at ? [...Array.from(at)] : [];
  if (!subs.value.some((s) => s[0] === pick.sub)) pick.sub = 'off';
}

function openPick(e: MouseEvent, which: 'sub' | 'audio') {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const items = which === 'sub'
    ? [{ head: t('viewer.media.subs') }, ...subs.value.map(([v, l]) => ({ label: l, radio: true, checked: () => pick.sub === v, run: () => { pick.sub = v; } }))]
    : [{ head: t('viewer.media.audio') }, ...audios.value.map((a, k) => ({
      label: a.label || a.language || t('viewer.media.track', { n: k + 1 }), radio: true, checked: () => pick.audio === k,
      run: () => { pick.audio = k; audios.value.forEach((x, i) => { x.enabled = i === k; }); },
    }))];
  // The shell closes menus on every click, so this one opens once the click is done.
  setTimeout(() => menu(r.left, r.top, items, { above: true }), 0);
}

const fullscreen = (e: MouseEvent) => (e.currentTarget as HTMLElement).requestFullscreen?.().catch(() => {});

onMounted(() => { if (props.autoplay) toggle(); });
defineExpose({ keys: (e: KeyboardEvent, combo: string) => (props.compact && !props.autoplay ? undefined : keys(e, combo)) });
</script>

<template>
  <button v-if="compact && !autoplay" class="cv-screen compact" data-run="view" :data-tip="t('viewer.media.playBig')" @click="run('view')">
    <img v-if="!noPoster" class="cv-poster" :src="poster" alt="" @error="noPoster = true"><span class="cv-big"><G n="IconPlay" /></span>
  </button>
  <template v-else-if="compact">
    <div class="cv-screen compact" :class="{ playing: st.playing }" data-play @click="toggle">
      <img v-if="!noPoster" class="cv-poster" :src="poster" alt="" @error="noPoster = true">
      <video ref="video" :class="{ seen }" :src="rawUrl(path)" preload="metadata" playsinline @loadedmetadata="metadata"></video>
      <span class="cv-big"><G :n="st.playing ? 'IconPause' : 'IconPlay'" /></span>
    </div>
    <div class="cv-vbar">
      <span class="glim-num cv-now">{{ fmtDur(st.t) }}</span>
      <span class="cv-scrub" role="slider" :aria-label="t('viewer.media.position')" :style="{ '--p': st.dur ? (st.t / st.dur * 100).toFixed(2) + '%' : '0%' }" @pointerdown="scrub"><i></i></span>
      <span class="glim-num muted">{{ fmtDur(st.dur) }}</span>
    </div>
  </template>
  <div v-else class="cv-video" :class="{ playing: st.playing }">
    <div class="cv-screen" data-play @click="toggle" @dblclick="fullscreen">
      <img v-if="!noPoster" class="cv-poster" :src="poster" alt="" @error="noPoster = true">
      <video ref="video" :class="{ seen }" :src="rawUrl(path)" preload="metadata" playsinline @loadedmetadata="metadata"></video>
      <div class="cv-vtitle"><b>{{ title.name }}</b><span class="glim-num">{{ title.year }}</span></div>
      <span class="cv-big"><G n="IconPlay" /></span>
      <div class="cv-sub">{{ line }}</div>
    </div>
    <div class="cv-ctrl">
      <button class="btn icon" data-play data-pi :data-tip="st.playing ? t('viewer.media.pauseTip') : t('viewer.media.playTip')" :aria-label="st.playing ? t('viewer.media.pause') : t('viewer.media.play')" @click="toggle">
        <G :n="st.playing ? 'IconPause' : 'IconPlay'" />
      </button>
      <span class="glim-num cv-now">{{ fmtDur(st.t) }}</span>
      <div class="cv-scrub" role="slider" :aria-label="t('viewer.media.position')" :aria-valuenow="Math.round(st.t)" :aria-valuemax="Math.round(st.dur)" tabindex="0"
        :style="{ '--p': st.dur ? (st.t / st.dur * 100).toFixed(2) + '%' : '0%' }" @pointerdown="scrub"><i></i><b></b></div>
      <span class="glim-num muted">{{ fmtDur(st.dur) }}</span>
      <button class="btn" data-pick="sub" :data-tip="t('viewer.media.subs')" @click="openPick($event, 'sub')"><G n="IconFileSubtitles" /><span>{{ subLabel }}</span></button>
      <button v-if="audios.length > 1" class="btn" data-pick="audio" :data-tip="t('viewer.media.audio')" @click="openPick($event, 'audio')">
        <G n="IconFileAudio" /><span>{{ audios[pick.audio]?.label || t('viewer.media.track', { n: pick.audio + 1 }) }}</span>
      </button>
    </div>
  </div>
</template>
