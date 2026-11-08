<script setup lang="ts">
import { Button, Card, ColorPicker, FieldRow, Selector, Toggle } from '@glimstone/vue';
import { ACCENTS, applyAccent, COLOR_NAMES_DE, DEFAULT_ACCENT, RAINBOW } from '@glimstone/core';
import { computed, ref } from 'vue';
import { G } from '@/core/glyphs';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { language as current } from '@/i18n';
import { found, motionPressed, rainbowSwitched, shapePressed } from '../eggs';
import { c, P } from '@/features/config/shared';
import { pickerWords, segItems } from '@/ui/kit';
import LanguagePicker from '../LanguagePicker.vue';

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

const rainbow = computed({ get: () => prefs.rainbow, set: (on: boolean) => { rainbowSwitched(on); setPref('rainbow', on); } });
// Turning rotation on draws a fresh offset at once, so the switch does something visible.
const rotate = computed({ get: () => prefs.rotate, set: (on: boolean) => { prefs.rotateSeed = on ? 1 + Math.floor(Math.random() * 7) : 0; setPref('rotate', on); } });

const accentDefault = computed(() => prefs.accent.toLowerCase() === DEFAULT_ACCENT.toLowerCase() && same(presets.value, PRESETS));
const ordered = computed(() => (prefs.tabOrder || []).length > 0);

// The picker floats beside the swatch it belongs to and paints the page live while it is dragged. What it shows when
// it closes is saved, however it was closed.
const pop = ref<{ anchor: HTMLElement; hex: string; live: (hex: string) => void; done: (hex: string) => void } | null>(null);
let shown = '';
function live(hex: string) {
  shown = hex.toUpperCase();
  pop.value?.live(shown);
}
function closed() {
  if (shown) pop.value?.done(shown);
  shown = '';
  pop.value = null;
}

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
    <FieldRow :label="c('look.language')"><LanguagePicker /></FieldRow>
    <FieldRow :label="c('look.theme')" :hint="c('look.themeTip')"><Selector v-model="P.theme" :items="segItems([['dark', c('look.dark'), 'IconMoon'], ['light', c('look.light'), 'IconSun'], ['system', c('look.system'), 'IconThemeSystem']])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="c('look.corners')" :hint="c('look.cornersTip')"><Selector id="a-shape" :model-value="prefs.shape" :items="segItems(shapes)" size="sm" activation="manual" inline button-height pairs-on-phone @press="(v) => setPref('shape', shapePressed(v, prefs.shape))" /></FieldRow>
    <FieldRow :label="c('look.motion')" :hint="c('look.motionTip')"><Selector id="a-motion" :model-value="prefs.motion" :items="segItems(motions)" size="sm" activation="manual" inline button-height pairs-on-phone @press="(v) => setPref('motion', motionPressed(v, prefs.motion))" /></FieldRow>
    <FieldRow :label="c('look.quiet')" :hint="c('look.quietTip')"><Toggle v-model="P.quietToasts" /></FieldRow>
    <FieldRow :label="c('look.order')" :hint="c('look.orderTip')"><Button id="a-order-reset" :label="c('look.reset')" label-key="IconRefresh" :disabled="!ordered" @click="resetOrder" /></FieldRow>
  </Card>
  <Card :title="c('look.labels')">
    <FieldRow :label="c('look.buttons')" :hint="c('look.buttonsTip')"><Selector v-model="P.labelsButtons" :items="segItems(labelModes)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="c('look.sidebar')"><Selector v-model="P.labelsSidebar" :items="segItems(labelModes)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="c('look.tabs')" :hint="c('look.tabsTip')"><Selector v-model="P.labelsTabs" :items="segItems(labelModes)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="c('look.bottombar')" :hint="c('look.bottombarTip')"><Selector v-model="P.labelsBottombar" :items="segItems(labelModes)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
  </Card>
  <Card :title="c('look.colours')">
    <!-- While the rainbow owns the colours the accent row stays visible but dimmed, and its bubble says how to get it back. -->
    <FieldRow class="a-accent" :class="{ dim: prefs.rainbow }" :label="c('look.accent')" :hint="prefs.rainbow ? c('look.accentDimTip') : undefined">
      <template #label><span class="a-dimmable">{{ c('look.accent') }}</span></template>
      <div class="a-swatches a-dimmable">
        <button v-for="(hex, i) in presets" :key="i" class="a-swatch" :style="{ '--c': hex }" :data-preset="i" :aria-pressed="hex.toLowerCase() === prefs.accent.toLowerCase()" :disabled="prefs.rainbow" :data-tip="c('look.preset', { n: i + 1 })" :aria-label="c('look.preset', { n: i + 1 })" @click="preset($event, i)"></button>
        <button id="a-accent-reset" class="a-reset" :data-tip="c('look.accentReset')" :aria-label="c('look.accentReset')" :disabled="accentDefault || prefs.rainbow" @click="resetAccent"><G n="IconResetArrow" /></button>
      </div>
    </FieldRow>
    <FieldRow :label="c('look.rainbow')" :hint="c('look.rainbowTip')"><Toggle id="a-rainbow" v-model="rainbow" /></FieldRow>
    <!-- Everything under the rainbow only shows while it is on: a palette editor under a rainbow that is not running offers edits with no effect. -->
    <template v-if="prefs.rainbow">
      <FieldRow v-if="found.disco || prefs.disco" :label="c('look.disco')" :hint="c('look.discoTip')"><Toggle v-model="P.disco" data-pref="disco" /></FieldRow>
      <FieldRow :label="c('look.reactiveMode')" :hint="c('look.reactiveTip')"><Toggle v-model="P.reactive" /></FieldRow>
      <FieldRow :label="c('look.rotate')" :hint="c('look.rotateTip')"><Toggle id="a-rotate" v-model="rotate" /></FieldRow>
      <FieldRow :label="c('look.palette')"><div class="a-swatches">
        <button v-for="(hex, i) in palette" :key="i" class="a-swatch" :style="{ '--c': hex }" :data-palette="i" :data-tip="c('look.paletteN', { n: i + 1 })" :aria-label="c('look.paletteN', { n: i + 1 })" @click="slot($event, i)"></button>
        <button id="a-palette-reset" class="a-reset" :data-tip="c('look.paletteReset')" :aria-label="c('look.paletteReset')" :disabled="same(palette, RAINBOW)" @click="setPref('palette', [...RAINBOW])"><G n="IconResetArrow" /></button>
      </div></FieldRow>
    </template>
  </Card>
  <ColorPicker v-if="pop" :key="pop.anchor.dataset.preset ?? 'p' + pop.anchor.dataset.palette" :model-value="pop.hex" :anchor="pop.anchor" :names="current() === 'de' ? COLOR_NAMES_DE : false" :text="pickerWords()" @input="live" @close="closed" />
</template>
