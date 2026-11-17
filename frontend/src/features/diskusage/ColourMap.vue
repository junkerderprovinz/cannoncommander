<script setup lang="ts">
// The treemap coloured by file type or by age: each folder a group of its own largest entries, every tile in the
// colour of the type or the age that holds most of its bytes. A click goes into the folder, Ctrl or Shift picks it.
import { computed, ref } from 'vue';
import type { UsageEntry } from '@/api/tools';
import { fmtSize } from '@/core/format';
import { join } from '@/files/paths';
import { plural, t } from '@/i18n';
import { AGES, ageKey, type ColourMode, dimmed, fillOf, inkOf, keyOf, typeMix } from './colours';
import PointBubble from './PointBubble.vue';
import { fit, type MapTile, nestedMap, pct } from './treemap';

const props = defineProps<{ items: UsageEntry[]; path: string; mode: Exclude<ColourMode, 'standard'>; hl: string | null; picked: Set<string>; w: number; h: number }>();
const emit = defineEmits<{ go: [path: string]; pick: [i: number, e: MouseEvent] }>();

const map = computed(() => nestedMap(props.items, props.w, props.h));
const here = computed(() => props.path.split('/').pop() || '/');

const look = (tile: MapTile) => {
  if (tile.rest) return { fill: 'var(--carbon-surface3)', ink: 'var(--carbon-text)' };
  const k = keyOf(props.mode, tile.e);
  return { fill: fillOf(props.mode, k), ink: inkOf(props.mode, k) };
};
const tiles = computed(() => map.value.tiles.map((tile, n) => {
  const g = map.value.groups[tile.group];
  return {
    tile, n, at: g ? join(props.path, g.e.name) : props.path, ...look(tile),
    dim: tile.rest ? props.hl != null : dimmed(props.mode, tile.e, props.hl),
    sel: tile.group < 0 && props.picked.has(join(props.path, tile.e.name)),
    name: fit(tile.rest ? plural(tile.rest, 'diskusage.more') : tile.e.name, tile.w - 12), size: fit(fmtSize(tile.e.size), tile.w - 12),
  };
}));
const groups = computed(() => map.value.groups.map((g) => ({ g, sel: props.picked.has(join(props.path, g.e.name)), name: fit(g.e.name, g.w - 90), size: fmtSize(g.e.size) })));

function click(e: MouseEvent) {
  const el = (e.target as Element).closest<SVGGElement>('[data-t], [data-g]');
  if (!el) return;
  const tile = el.dataset.t != null ? map.value.tiles[Number(el.dataset.t)]! : null;
  const group = map.value.groups[tile ? tile.group : Number(el.dataset.g)];
  const entry = group?.e ?? tile!.e;
  const i = group ? group.i : tile!.i!;
  if (entry.dir && !(e.ctrlKey || e.metaKey || e.shiftKey)) emit('go', join(props.path, entry.name));
  else emit('pick', i, e);
}

const hover = ref<{ x: number; y: number; n: number } | null>(null);
function move(e: PointerEvent) {
  const el = (e.target as Element).closest<SVGGElement>('[data-t]');
  hover.value = el ? { x: e.clientX, y: e.clientY, n: Number(el.dataset.t) } : null;
}
const tip = computed(() => {
  const x = hover.value && tiles.value[hover.value.n];
  if (!x) return null;
  const { tile } = x, e = tile.e, g = map.value.groups[tile.group];
  const whole = g ? g.e.size : props.items.reduce((a, it) => a + it.size, 0);
  const mix = typeMix(e.kinds), sum = mix.reduce((a, [, v]) => a + v, 0);
  return {
    name: tile.rest ? plural(tile.rest, 'diskusage.more') : e.name,
    path: tile.rest ? x.at : join(x.at, e.name),
    size: `${fmtSize(e.size) || '0 B'}, ${t('diskusage.tipShare', { pct: Math.round(pct(e.size, whole)), folder: g ? g.e.name : here.value })}`,
    mix: mix.map(([k, v]) => ({ k, f: v / sum })),
    age: e.ages?.some((v) => v > 0) ? t('diskusage.tipAge', { age: t(`diskusage.ages.${AGES[Number(ageKey(e))]}`) }) : '',
  };
});
</script>

<template>
  <svg :width="w" :height="h" :viewBox="`0 0 ${w} ${h}`" :aria-label="t('diskusage.mapLabel', { path })" @click="click" @pointermove="move" @pointerleave="hover = null">
    <g v-for="(x, k) in groups" :key="x.g.e.name" class="du-group" :class="{ sel: x.sel }" :data-g="k" :data-name="x.g.e.name">
      <rect :x="x.g.x" :y="x.g.y" :width="x.g.w" :height="x.g.h" rx="7"></rect>
      <template v-if="x.g.head">
        <text :x="x.g.x + 9" :y="x.g.y + 18" class="n">{{ x.name }}</text>
        <text :x="x.g.x + x.g.w - 9" :y="x.g.y + 18" class="s" text-anchor="end">{{ x.size }}</text>
      </template>
    </g>
    <g v-for="x in tiles" :key="x.n" class="du-tile" :class="{ dim: x.dim, sel: x.sel }" :data-t="x.n" :data-name="x.tile.e.name" :style="{ '--blk': x.fill, '--ink': x.ink }">
      <rect :x="x.tile.x" :y="x.tile.y" :width="x.tile.w" :height="x.tile.h" rx="4"></rect>
      <text v-if="x.tile.w > 44 && x.tile.h > 20 && x.name" :x="x.tile.x + 6" :y="x.tile.y + 15" class="n">{{ x.name }}</text>
      <text v-if="x.tile.w > 44 && x.tile.h > 36 && x.size" :x="x.tile.x + 6" :y="x.tile.y + 30" class="s">{{ x.size }}</text>
    </g>
  </svg>
  <PointBubble :at="hover">
    <div v-if="tip" class="du-tipcard">
      <b>{{ tip.name }}</b>
      <span class="path">{{ tip.path }}</span>
      <span>{{ tip.size }}</span>
      <template v-if="tip.mix.length">
        <span class="du-stack"><i v-for="m in tip.mix" :key="m.k" :style="{ width: `${m.f * 100}%`, background: fillOf('type', m.k) }"></i></span>
        <span class="du-mix"><template v-for="m in tip.mix.slice(0, 3)" :key="m.k"><i :style="{ background: fillOf('type', m.k) }"></i><span>{{ t(`diskusage.groups.${m.k}`) }}</span><span class="glim-num">{{ Math.round(m.f * 100) }} %</span></template></span>
      </template>
      <span v-if="tip.age" class="muted">{{ tip.age }}</span>
    </div>
  </PointBubble>
</template>
