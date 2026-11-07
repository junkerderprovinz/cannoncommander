<script setup lang="ts">
// One entry in any view. Every view draws the same .row with its index, so clicks, marks, drags and menus work the
// same in all of them; only the inside differs.
import { Meter, Pill, RowMark } from '@glimstone/vue';
import { computed } from 'vue';
import { extend } from '@/core/extend';
import { fmtListDate, fmtSize } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import { Render } from '@/core/Render';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import type { RowMark as Mark } from '@/stores/panels';
import { COL, Columns } from './columns';
import type { Entry } from './entries';
import { join } from './paths';
import SizeText from './SizeText.vue';
import Thumb from './Thumb.vue';
import { fresh } from './fresh';
import { ticking } from './touch';

const props = defineProps<{ k: Entry; idx: number; view: string; path: string; marked: boolean; cursor: boolean; cut?: boolean; mark?: Mark }>();
const listing = useListing();
const boxes = computed(() => ticking(props.path) && !props.k.up);
const glows = computed(() => fresh.has(join(props.k.at || props.path, props.k.name)));

const kind = computed(() => (['details', 'compact'].includes(props.view) ? 'row'
  : ['list', 'columns'].includes(props.view) ? 'flow'
    : props.view === 'content' ? 'content' : props.view === 'drives' ? 'drive' : 'tile'));
const cls = computed(() => [
  { row: 'row', flow: 'row fitem', content: 'row citem', tile: 'row gcard', drive: 'row dcard' }[kind.value],
  { dir: props.k.dir, hidden: props.k.hidden || props.k.name.startsWith('.'), marked: props.marked, cursor: props.cursor, cut: props.cut, fresh: glows.value },
]);
const folder = computed(() => props.k.at || props.path);
const shownName = computed(() => {
  const k = props.k;
  return prefs.showExt || k.dir || !k.name.includes('.', 1) ? k.name : k.name.slice(0, k.name.lastIndexOf('.'));
});
const objects = computed(() => (props.k.dir ? t('files.objShort', { n: props.k.items ?? 0 }) : fmtSize(props.k.size)));
// The prototype calls a failure fault, GlimStone fail.
const toneOf = (tone?: string) => (tone === 'fault' ? 'fail' : tone === 'ok' || tone === 'warn' ? tone : 'state');

// A folder where a volume is mounted shows how full the volume is instead of its size.
const vol = computed(() => (props.k.dir ? listing.placeOf(join(props.path, props.k.name)) : undefined));
const volume = computed(() => (vol.value?.free != null && vol.value.total ? vol.value as { free: number; total: number } : null));
const driveGlyph = computed(() => {
  const p = join(props.path, props.k.name);
  return p === '/boot' ? 'IconFlash' : p === '/remotes' ? 'IconCloud' : p === '/mnt/user' ? 'IconFiles' : volume.value ? 'IconDatabase' : glyphOf(props.k);
});
const driveName = computed(() => (vol.value?.label ? `${vol.value.label} (${props.k.name})` : props.k.name));
const extra = computed(() => ['dimensions', 'duration', 'pages'].map((id) => COL[id]!.value(props.k, props.path)).find(Boolean));
</script>

<template>
  <div :class="cls" :data-idx="idx" draggable="true" :data-tip="kind === 'tile' ? k.name : undefined">
    <template v-if="kind === 'row' || kind === 'flow'">
      <template v-for="id in kind === 'row' ? Columns.visible() : ['name']" :key="id">
        <span v-if="id === 'name'" class="name">
          <span class="glyph"><RowMark v-if="marked || boxes" :on="marked" /><G v-else :n="glyphOf(k)" /></span><span class="t">{{ shownName }}</span>
          <Render v-for="(badge, n) in extend.nameBadges" :key="n" :node="badge(k, path)" />
          <span v-if="k.at && k.at !== path" class="link">{{ t('files.foundIn', { path: k.at.slice(path.length + 1) }) }}</span>
          <span v-if="k.link" class="link">→ {{ k.link }}</span>
          <Pill v-if="k.archive" tone="state">{{ t('files.archive') }}</Pill>
          <Pill v-if="mark" :tone="toneOf(mark.tone)">{{ mark.label }}</Pill>
        </span>
        <span v-else-if="id === 'size' && volume" class="cell r glim-num vcell"><Meter class="vmeter" size="small" :value="1 - volume.free / volume.total" :label="driveName" :value-text="t('files.freeOf', { free: fmtSize(volume.free), total: fmtSize(volume.total) })" />{{ t('files.free', { size: fmtSize(volume.free) }) }}</span>
        <span v-else-if="id === 'size' && k.sizeUnknown && prefs.folderSizes" class="cell r glim-num"><SizeText :k="k" /></span>
        <span v-else class="cell glim-num" :class="{ r: COL[id]!.r, mono: COL[id]!.mono }">{{ COL[id]!.value(k, path) }}</span>
      </template>
    </template>
    <template v-else-if="kind === 'tile'">
      <Thumb :k="k" :folder="folder" /><RowMark v-if="marked || boxes" :on="marked" />
      <span class="t">{{ k.name }}</span><span class="meta glim-num">{{ objects }}<Render v-for="(badge, n) in extend.nameBadges" :key="n" :node="badge(k, path)" /></span>
    </template>
    <template v-else-if="kind === 'content'">
      <Thumb :k="k" :folder="folder" /><RowMark v-if="marked || boxes" :on="marked" />
      <span class="c-main"><span class="t">{{ k.name }}</span><span class="c-sub">{{ COL.type!.value(k, path) }} · {{ fmtListDate(k.time) }}</span></span>
      <span class="c-side glim-num"><span>{{ objects }}</span><span v-if="extra">{{ extra }}</span></span>
    </template>
    <template v-else>
      <RowMark v-if="marked" on /><span class="dglyph"><G :n="driveGlyph" /></span>
      <span class="dtext"><span class="t">{{ driveName }}</span>
        <template v-if="volume"><Meter class="dbar" size="medium" :value="1 - volume.free / volume.total" :label="driveName" :value-text="t('files.freeOf', { free: fmtSize(volume.free), total: fmtSize(volume.total) })" /><span class="meta glim-num">{{ t('files.freeOf', { free: fmtSize(volume.free), total: fmtSize(volume.total) }) }}</span></template>
        <span v-else class="meta glim-num">{{ objects }}</span>
      </span>
    </template>
  </div>
</template>
