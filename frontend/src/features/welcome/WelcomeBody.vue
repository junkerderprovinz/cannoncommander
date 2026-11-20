<script setup lang="ts">
// The greeting: the logo arrives and its cannons fire once, then the ways to lay out the app come in as cards. A card
// takes effect on the pick, from the pointer or from the arrow keys, and Enter also closes the window.
import { nextTick, ref, useId } from 'vue';
import { glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { choices, choose, chosen, type Way, welcomeLogo } from './welcome';

const props = defineProps<{ close: () => void }>();
const lead = useId();
const logo = welcomeLogo();
const list = choices();
const picked = ref<Way>(chosen());
const cards = ref<HTMLButtonElement[]>([]);
const Tick = glyphComponent('IconCheck');

// The sketch of a window at work: the path on top, the sidebar, and one or two panels with a few rows. In one window
// the cursor walks down and a folder opens, in two a file crosses from the left panel to the right one.
const W = 160;
const H = 100;
const LEFT = 44;
const GAP = 4;
const panels = (way: Way) => {
  const n = way === 'single' ? 1 : 2;
  const w = (W - 8 - LEFT - GAP * (n - 1)) / n;
  return Array.from({ length: n }, (_, i) => ({ x: LEFT + i * (w + GAP), w }));
};
// Each row's place, how much of the panel's width its name takes, and how wide it is in the folder that opens.
const ROWS = [[30, .85, .6], [38, .6, 1.4], [46, .75, .8], [54, .5, 1.6], [62, .7, .9]] as const;

function pick(way: Way) {
  picked.value = way;
  choose(way);
}

function onKey(e: KeyboardEvent, i: number) {
  const n = list.length;
  const rtl = getComputedStyle(e.currentTarget as Element).direction === 'rtl';
  const step = { ArrowDown: 1, ArrowUp: -1, ArrowRight: rtl ? -1 : 1, ArrowLeft: rtl ? 1 : -1 }[e.key];
  let to: number | undefined;
  if (step) to = (i + step + n) % n;
  else if (e.key === 'Home') to = 0;
  else if (e.key === 'End') to = n - 1;
  else if (e.key === 'Enter') {
    e.preventDefault();
    pick(list[i]!.id);
    props.close();
    return;
  } else return;
  e.preventDefault();
  pick(list[to]!.id);
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
          <rect class="wl-sk-crumb" x="11" y="10.5" width="22" height="3" rx="1.5" />
          <rect class="wl-sketch-panel" x="8" y="22" width="30" height="70" rx="3" />
          <rect v-for="[y] in ROWS.slice(0, 3)" :key="y" class="wl-sketch-row" x="13" :y="y" width="18" height="3" rx="1.5" />
          <g v-for="(p, k) in panels(c.id)" :key="k">
            <rect class="wl-sketch-panel" :x="p.x" y="22" :width="p.w" height="70" rx="3" />
            <rect v-if="k === 0" class="wl-sk-cursor" :x="p.x + 3" y="28" :width="p.w - 6" height="7" rx="2" />
            <template v-if="k === 0">
              <rect v-for="(row, r) in ROWS.slice(0, 3)" :key="row[0]" class="wl-sk-glow" :x="p.x + 3" :y="row[0] - 2" :width="p.w - 6" height="7" rx="2" :style="{ '--wl-r': r }" />
            </template>
            <rect
              v-for="([y, share, wide], r) in ROWS" :key="y" class="wl-sketch-row" :class="{ 'wl-sk-row': c.id === 'single' }"
              :x="p.x + 6" :y="y" :width="(p.w - 12) * share" height="3" rx="1.5" :style="{ '--wl-r': r, '--wl-wide': wide }"
            />
            <template v-if="c.id === 'single'">
              <rect v-for="[y, share, wide] in ROWS" :key="y" class="wl-sk-alt" :x="p.x + 6" :y="y" :width="(p.w - 12) * share * wide" height="3" rx="1.5" />
            </template>
            <rect v-if="k === 1" class="wl-sk-new" :x="p.x + 6" y="70" :width="(p.w - 12) * .65" height="3" rx="1.5" />
          </g>
          <rect v-if="c.id === 'dual'" class="wl-sk-chip" :x="LEFT + 4" y="37" width="30" height="5" rx="2" />
        </svg>
        <span v-if="c.id === picked" class="wl-tick"><Tick /></span>
        <span class="wl-name">{{ c.name }}</span>
        <span class="wl-how">{{ c.how }}</span>
      </button>
    </div>
    <p class="wl-hint">{{ t('welcome.hint') }}</p>
  </div>
</template>
