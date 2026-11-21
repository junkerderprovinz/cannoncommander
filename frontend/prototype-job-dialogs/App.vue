<script setup lang="ts">
// Prototype: seven layouts of the copy, move and delete windows in every state, switchable by ?variant= and the bar.
import { applyOled } from '@glimstone/core';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { G } from '@/core/glyphs';
import Cell from './Cell.vue';
import Inspiration from './Inspiration.vue';
import { type Dialog, DIALOGS, type Lang, type State, STATES, texts } from './data';

const VARIANTS = ['G', 'A', 'B', 'C', 'D', 'E', 'F'] as const;
const params = new URLSearchParams(location.search);
const variant = ref(VARIANTS.includes(params.get('variant') as never) ? params.get('variant')! : 'G');
const lang = ref<Lang>(params.get('lang') === 'en' ? 'en' : 'de');
const mode = ref(params.get('mode') ?? 'dark');
const MOTIONS = ['off', 'subtle', 'wild', 'storm'] as const;
const motion = ref(MOTIONS.includes(params.get('motion') as never) ? params.get('motion')! : 'subtle');
const dialog = ref<Dialog | 'all'>('all');
const state = ref<State | 'all'>('all');
const t = computed(() => texts(lang.value));

const shown = computed(() => DIALOGS.filter((d) => dialog.value === 'all' || d === dialog.value)
  .flatMap((d) => STATES.filter((s) => state.value === 'all' || s === state.value).map((s) => ({ d, s }))));

function sync() {
  const q = new URLSearchParams({ variant: variant.value, lang: lang.value, mode: mode.value, motion: motion.value });
  history.replaceState(null, '', `${location.pathname}?${q}`);
  const root = document.documentElement;
  root.dataset.theme = mode.value === 'light' ? 'light' : 'dark';
  applyOled(mode.value === 'oled');
  root.lang = lang.value;
  root.dataset.motion = motion.value;
}
watch([variant, lang, mode, motion], sync, { immediate: true });
const step = (d: number) => { variant.value = VARIANTS[(VARIANTS.indexOf(variant.value as never) + d + VARIANTS.length) % VARIANTS.length]!; };
function key(e: KeyboardEvent) {
  if ((e.target as Element).closest?.('input, textarea, [contenteditable]')) return;
  if (e.key === 'ArrowLeft') step(-1);
  if (e.key === 'ArrowRight') step(1);
}
onMounted(() => addEventListener('keydown', key));
onBeforeUnmount(() => removeEventListener('keydown', key));
</script>

<template>
  <main class="proto-page">
    <header class="proto-head" :data-variant-intro="variant">
      <h1>{{ lang === 'de' ? 'Aufgabenfenster' : 'Job windows' }} · {{ variant }}: {{ t.variant[variant as 'A'] }}</h1>
      <div class="proto-filters">
        <button :class="{ on: dialog === 'all' }" @click="dialog = 'all'">{{ t.all }}</button>
        <button v-for="d in DIALOGS" :key="d" :class="{ on: dialog === d }" @click="dialog = d">{{ t.dialog[d] }}</button>
        <span class="proto-gap"></span>
        <button :class="{ on: state === 'all' }" @click="state = 'all'">{{ t.all }}</button>
        <button v-for="s in STATES" :key="s" :class="{ on: state === s }" @click="state = s">{{ t.state[s] }}</button>
      </div>
    </header>
    <section class="proto-grid">
      <figure v-for="{ d, s } in shown" :key="`${variant}-${d}-${s}-${lang}`" class="proto-cell" :data-cell="`${d}-${s}`">
        <figcaption>{{ t.dialog[d] }} · {{ t.state[s] }}</figcaption>
        <Cell :variant="variant" :dialog="d" :state="s" :lang="lang" />
      </figure>
    </section>
    <Inspiration />
  </main>
  <nav class="proto-bar" aria-label="Prototyp">
    <button :aria-label="lang === 'de' ? 'Vorige Variante' : 'Previous variant'" @click="step(-1)"><G n="IconBack" /></button>
    <span class="proto-label" data-variant-label><b>{{ variant }}</b> {{ t.variant[variant as 'A'] }}</span>
    <button :aria-label="lang === 'de' ? 'Nächste Variante' : 'Next variant'" @click="step(1)"><G n="IconForward" /></button>
    <span class="proto-sep"></span>
    <button v-for="m in ['light', 'dark', 'oled']" :key="m" :class="{ on: mode === m }" :data-mode="m" @click="mode = m">{{ t[m as 'light'] }}</button>
    <span class="proto-sep"></span>
    <button v-for="m in MOTIONS" :key="m" :class="{ on: motion === m }" :data-motion-pick="m" :aria-label="`${t.motionLabel}: ${t.motion[m]}`" @click="motion = m">{{ t.motion[m] }}</button>
    <span class="proto-sep"></span>
    <button v-for="l in ['de', 'en']" :key="l" :class="{ on: lang === l }" :data-lang="l" @click="lang = l as Lang">{{ l.toUpperCase() }}</button>
  </nav>
</template>
