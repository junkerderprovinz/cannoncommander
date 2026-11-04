<script setup lang="ts">
import { ACCENTS, applyAccent, DEFAULT_ACCENT, RAINBOW } from '@glimstone/core';
import { computed, ref } from 'vue';
import { G } from '@/core/glyphs';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { setLanguage } from '@/i18n';
import { found, motionPressed, rainbowSwitched, shapePressed } from '../eggs';
import Card from '../kit/Card.vue';
import ColourPop from '../kit/ColourPop.vue';
import Dropdown from '../kit/Dropdown.vue';
import Frow from '../kit/Frow.vue';
import Info from '../kit/Info.vue';
import { c, P } from '../kit/kit';
import Seg from '../kit/Seg.vue';
import Sw from '../kit/Sw.vue';

const PRESETS = ACCENTS.map((a) => a.hex);
const same = (a: string[], b: string[]) => a.length === b.length && a.every((x, i) => x.toLowerCase() === b[i]!.toLowerCase());
const valid = (list: unknown): list is string[] => Array.isArray(list) && list.length === 8 && list.every((h) => /^#[0-9a-f]{6}$/i.test(h));
const presets = computed(() => (valid(prefs.accentPresets) ? prefs.accentPresets : PRESETS));
const palette = computed(() => (valid(prefs.palette) ? prefs.palette : RAINBOW));
const seed = () => (prefs.rotate ? prefs.rotateSeed || 0 : 0);

const shapes = computed(() => [['round', c('look.round'), 'IconCornerRound'], ['soft', c('look.soft'), 'IconCornerSoft'], ['square', c('look.square'), 'IconCornerSquare'],
  ...(prefs.shape === 'leaf' || found.leaf ? [['leaf', c('look.leaf'), 'IconCornerLeaf']] : [])] as [string, string, string][]);
const motions = computed(() => [['off', c('look.off'), 'IconMotionOff'], ['subtle', c('look.subtle'), 'IconMotionSubtle'], ['wild', c('look.wild'), 'IconMotionWild'],
  ...(prefs.motion === 'storm' || found.storm ? [['storm', c('look.storm'), 'IconMotionStorm']] : [])] as [string, string, string][]);
const labelModes = computed(() => [['text', c('look.text'), 'IconLabelText'], ['textGlyph', c('look.textGlyph'), 'IconLabelTextGlyph'], ['glyph', c('look.glyph'), 'IconLabelGlyph'], ['reactive', c('look.reactive'), 'IconLabelReactive']] as [string, string, string][]);

const language = computed({ get: () => prefs.language, set: (v: string) => { setPref('language', v); setLanguage(v); } });
const rainbow = computed({ get: () => prefs.rainbow, set: (on: boolean) => { rainbowSwitched(on); setPref('rainbow', on); } });
// Turning rotation on draws a fresh offset at once, so the switch does something visible.
const rotate = computed({ get: () => prefs.rotate, set: (on: boolean) => { prefs.rotateSeed = on ? 1 + Math.floor(Math.random() * 7) : 0; setPref('rotate', on); } });

const accentDefault = computed(() => prefs.accent.toLowerCase() === DEFAULT_ACCENT.toLowerCase() && same(presets.value, PRESETS));
const ordered = computed(() => (prefs.tabOrder || []).length > 0);

// The picker floats beside the swatch it belongs to and paints the page live while it is dragged.
const pop = ref<{ anchor: HTMLElement; hex: string; live: (hex: string) => void; done: (hex: string) => void } | null>(null);

function preset(e: MouseEvent, i: number) {
  const el = e.currentTarget as HTMLElement;
  setPref('accent', presets.value[i]);
  pop.value = {
    anchor: el, hex: presets.value[i]!,
    live: (hex) => { el.style.setProperty('--c', hex); applyAccent(hex); },
    done: (hex) => { const next = [...presets.value]; next[i] = hex; prefs.accentPresets = next; setPref('accent', hex); },
  };
}
function slot(e: MouseEvent, i: number) {
  const el = e.currentTarget as HTMLElement;
  pop.value = {
    anchor: el, hex: palette.value[i]!,
    live: (hex) => { el.style.setProperty('--c', hex); document.documentElement.style.setProperty(`--rb-${(i - seed() + 8) % 8}`, hex); },
    done: (hex) => { const next = [...palette.value]; next[i] = hex; setPref('palette', next); },
  };
}
function resetAccent() {
  prefs.accentPresets = [...PRESETS];
  setPref('accent', DEFAULT_ACCENT);
}
function resetOrder() {
  setPref('tabOrder', []);
  toast(c('look.orderReset'));
}
</script>

<template>
  <Card :title="c('look.display')">
    <Frow :label="c('look.language')"><Dropdown v-model="language" :options="[['de', '🇩🇪 Deutsch'], ['en', '🇬🇧 English']]" /></Frow>
    <Frow :label="c('look.theme')" :tip="c('look.themeTip')"><Seg v-model="P.theme" :options="[['dark', c('look.dark'), 'IconMoon'], ['light', c('look.light'), 'IconSun'], ['system', c('look.system'), 'IconThemeSystem']]" /></Frow>
    <Frow :label="c('look.corners')" :tip="c('look.cornersTip')"><Seg id="a-shape" :model-value="prefs.shape" :options="shapes" @press="(v) => setPref('shape', shapePressed(v, prefs.shape))" /></Frow>
    <Frow :label="c('look.motion')" :tip="c('look.motionTip')"><Seg id="a-motion" :model-value="prefs.motion" :options="motions" @press="(v) => setPref('motion', motionPressed(v, prefs.motion))" /></Frow>
    <Frow :label="c('look.quiet')" :tip="c('look.quietTip')"><Sw v-model="P.quietToasts" /></Frow>
    <Frow :label="c('look.order')" :tip="c('look.orderTip')"><button id="a-order-reset" class="btn" :disabled="!ordered" @click="resetOrder"><G n="IconRefresh" />{{ c('look.reset') }}</button></Frow>
  </Card>
  <Card :title="c('look.labels')">
    <Frow :label="c('look.buttons')" :tip="c('look.buttonsTip')"><Seg v-model="P.labelsButtons" :options="labelModes" /></Frow>
    <Frow :label="c('look.sidebar')"><Seg v-model="P.labelsSidebar" :options="labelModes" /></Frow>
    <Frow :label="c('look.tabs')" :tip="c('look.tabsTip')"><Seg v-model="P.labelsTabs" :options="labelModes" /></Frow>
    <Frow :label="c('look.bottombar')" :tip="c('look.bottombarTip')"><Seg v-model="P.labelsBottombar" :options="labelModes" /></Frow>
  </Card>
  <Card :title="c('look.colours')">
    <!-- While the rainbow owns the colours the accent row stays visible but dimmed, and its bubble says how to get it back. -->
    <div class="frow a-accent" :class="{ dim: prefs.rainbow }"><span class="lab"><span class="a-dimmable">{{ c('look.accent') }}</span><template v-if="prefs.rainbow">{{ ' ' }}<Info :tip="c('look.accentDimTip')" /></template></span>
      <div class="a-swatches a-dimmable">
        <button v-for="(hex, i) in presets" :key="i" class="a-swatch" :style="{ '--c': hex }" :data-preset="i" :aria-pressed="hex.toLowerCase() === prefs.accent.toLowerCase()" :disabled="prefs.rainbow" :data-tip="c('look.preset', { n: i + 1 })" :aria-label="c('look.preset', { n: i + 1 })" @click="preset($event, i)"></button>
        <button id="a-accent-reset" class="a-reset" :data-tip="c('look.accentReset')" :aria-label="c('look.accentReset')" :disabled="accentDefault || prefs.rainbow" @click="resetAccent"><G n="IconResetArrow" /></button>
      </div>
    </div>
    <Frow :label="c('look.rainbow')" :tip="c('look.rainbowTip')"><Sw id="a-rainbow" v-model="rainbow" /></Frow>
    <!-- Everything under the rainbow only shows while it is on: a palette editor under a rainbow that is not running offers edits with no effect. -->
    <template v-if="prefs.rainbow">
      <Frow v-if="found.disco || prefs.disco" :label="c('look.disco')" :tip="c('look.discoTip')"><Sw v-model="P.disco" /></Frow>
      <Frow :label="c('look.reactiveMode')" :tip="c('look.reactiveTip')"><Sw v-model="P.reactive" /></Frow>
      <Frow :label="c('look.rotate')" :tip="c('look.rotateTip')"><Sw id="a-rotate" v-model="rotate" /></Frow>
      <div class="frow"><span class="lab">{{ c('look.palette') }}</span><div class="a-swatches">
        <button v-for="(hex, i) in palette" :key="i" class="a-swatch" :style="{ '--c': hex }" :data-palette="i" :data-tip="c('look.paletteN', { n: i + 1 })" :aria-label="c('look.paletteN', { n: i + 1 })" @click="slot($event, i)"></button>
        <button id="a-palette-reset" class="a-reset" :data-tip="c('look.paletteReset')" :aria-label="c('look.paletteReset')" :disabled="same(palette, RAINBOW)" @click="setPref('palette', [...RAINBOW])"><G n="IconResetArrow" /></button>
      </div></div>
    </template>
  </Card>
  <ColourPop v-if="pop" :key="pop.anchor.dataset.preset ?? 'p' + pop.anchor.dataset.palette" :anchor="pop.anchor" :hex="pop.hex" @live="pop.live" @done="pop.done" @close="pop = null" />
</template>
