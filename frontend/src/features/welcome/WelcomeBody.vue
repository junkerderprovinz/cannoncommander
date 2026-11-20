<script setup lang="ts">
// The greeting: the logo arrives and its cannons fire once, then the ways to lay out the app come in as cards. A card
// takes effect on the pick, from the pointer or from the arrow keys, and Enter also closes the window.
import { computed, nextTick, ref, useId } from 'vue';
import { glyphComponent } from '@/core/glyphs';
import { host } from '@/core/host';
import { t } from '@/i18n';
import { choices, choose, chosen, type Way, welcomeLogo } from './welcome';

const props = defineProps<{ close: () => void }>();
const lead = useId();
const logo = welcomeLogo();
const picked = ref<Way>(chosen());
const cards = ref<HTMLButtonElement[]>([]);

// The sketch of a window: the sidebar, one or two panels with a few rows, and the F-key bar where the way has one.
const W = 160;
const H = 100;
function sketch(panes: 1 | 2, fkeys: boolean) {
  const bottom = fkeys ? 80 : 92;
  const left = 44;
  const gap = 4;
  const width = (W - 8 - left - gap * (panes - 1)) / panes;
  return {
    bottom,
    panels: Array.from({ length: panes }, (_, i) => ({ x: left + i * (width + gap), w: width })),
    keys: fkeys ? Array.from({ length: 6 }, (_, i) => ({ x: 8 + i * 24.33, w: 20.33 })) : [],
  };
}
// Each row's place and how much of the panel's width its name takes.
const ROWS = [[30, .85], [38, .6], [46, .75], [54, .5], [62, .7]] as const;
const Tick = glyphComponent('IconCheck');
const list = computed(() => choices().map((c) => ({ ...c, s: sketch(c.panes, c.fkeys) })));

function pick(way: Way) {
  picked.value = way;
  choose(way);
}

function onKey(e: KeyboardEvent, i: number) {
  const n = list.value.length;
  const rtl = getComputedStyle(e.currentTarget as Element).direction === 'rtl';
  const step = { ArrowDown: 1, ArrowUp: -1, ArrowRight: rtl ? -1 : 1, ArrowLeft: rtl ? 1 : -1 }[e.key];
  let to: number | undefined;
  if (step) to = (i + step + n) % n;
  else if (e.key === 'Home') to = 0;
  else if (e.key === 'End') to = n - 1;
  else if (e.key === 'Enter') {
    e.preventDefault();
    pick(list.value[i]!.id);
    props.close();
    return;
  } else return;
  e.preventDefault();
  pick(list.value[to]!.id);
  void nextTick(() => cards.value[to!]?.focus());
}
</script>

<template>
  <div class="wl">
    <div class="wl-logo" v-html="logo"></div>
    <h2 :id="lead" class="wl-lead">{{ t('welcome.lead') }}</h2>
    <div class="wl-ways" role="radiogroup" :aria-labelledby="lead">
      <button
        v-for="(c, i) in list" :key="c.id" ref="cards" type="button" role="radio" class="wl-way"
        :class="{ 'wl-way--on': c.id === picked }" :aria-checked="c.id === picked" :tabindex="c.id === picked ? 0 : -1"
        :data-way="c.id" :style="{ '--wl-i': i }" @click="pick(c.id)" @keydown="onKey($event, i)"
      >
        <svg class="wl-sketch" :viewBox="`0 0 ${W} ${H}`" aria-hidden="true" focusable="false">
          <rect class="wl-sketch-frame" x="0" y="0" :width="W" :height="H" rx="9" />
          <rect class="wl-sketch-bar" x="8" y="8" :width="W - 16" height="8" rx="2.5" />
          <rect class="wl-sketch-panel" x="8" y="22" width="30" :height="c.s.bottom - 22" rx="3" />
          <rect v-for="[y] in ROWS.slice(0, 3)" :key="y" class="wl-sketch-row" x="13" :y="y" width="18" height="3" rx="1.5" />
          <g v-for="(p, k) in c.s.panels" :key="k">
            <rect class="wl-sketch-panel" :x="p.x" y="22" :width="p.w" :height="c.s.bottom - 22" rx="3" />
            <rect v-if="k === 0" class="wl-sketch-cursor" :x="p.x + 3" y="36" :width="p.w - 6" height="7" rx="2" />
            <rect v-for="[y, share] in ROWS" :key="y" class="wl-sketch-row" :x="p.x + 6" :y="y" :width="(p.w - 12) * share" height="3" rx="1.5" />
          </g>
          <rect v-for="(f, k) in c.s.keys" :key="k" class="wl-sketch-key" :x="f.x" y="85" :width="f.w" height="7" rx="2" />
        </svg>
        <span v-if="c.id === picked" class="wl-tick"><Tick /></span>
        <span class="wl-name">{{ c.name }}</span>
        <span class="wl-how">{{ c.how }}</span>
      </button>
    </div>
    <p class="wl-hint">{{ host.desktop ? t('welcome.hintDesktop') : t('welcome.hintWeb') }}</p>
  </div>
</template>
