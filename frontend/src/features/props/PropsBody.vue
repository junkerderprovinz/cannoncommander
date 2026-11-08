<script setup lang="ts">
import { Select } from '@glimstone/vue/Select';
import { Checkbox, FieldRow, GlyphTile, Pill, SubCard, TextField, Toggle } from '@glimstone/vue';
// Eigenschaften und Rechte: what the selection is, who owns it, and the nine rights as boxes and as a number.
import { computed, ref, watch } from 'vue';
import type { Item } from '@/api/client';
import { extend } from '@/core/extend';
import { fmtSize, fmtTime } from '@/core/format';
import { ext, kindOf, glyphComponent, glyphOf } from '@/core/glyphs';
import { Render } from '@/core/Render';
import { t } from '@/i18n';
import CopyBtn from '@/ui/CopyBtn.vue';
import { plural, pressPrimary, selectOptions } from '@/ui/kit';

export interface PropsState {
  nodes: Item[]; paths: string[]; dir: string; self: boolean; ro: boolean; uids: Record<string, number>; gids: Record<string, number>;
  modes: string[]; owners: string[]; groups: string[]; mode: string; owner: string; group: string; rec: boolean;
  size: number; files: number; dirs: number; counted: boolean; label: string;
  win: boolean; flags: Record<string, boolean>; flagsAt: Record<string, boolean>;
}
const props = defineProps<{ st: PropsState }>();
const st = props.st;
const one = computed(() => (st.nodes.length === 1 ? st.nodes[0]! : null));
const anyDir = st.nodes.some((k) => k.dir && !k.link);

// A selection with mixed owners keeps each one's own until a name is picked.
const pick = (known: Record<string, number>, cur: string[]) => {
  const names = [...new Set([...Object.keys(known), ...cur])].map((n) => [n, n] as const);
  return cur.length > 1 ? [['', t('props.keep')] as const, ...names] : names;
};
const owners = pick(st.uids, st.owners), groups = pick(st.gids, st.groups);
const idOf = (known: Record<string, number>, name: string, word: string) => (name ? `${word} ${known[name] ?? '?'}` : word);

const typeOf = (k: Item) => (k.link ? t(k.dir ? 'props.linkToDir' : 'props.linkToFile')
  : t(`props.kinds.${kindOf(k)}`) + (k.dir || !ext(k.name) ? '' : `, ${ext(k.name).toUpperCase()}`));
const bytes = computed(() => plural(Math.round(st.size), 'props.bytes'));
const sizeText = computed(() => {
  const size = fmtSize(st.size) || '0 B';
  if (!st.counted) return st.nodes.some((k) => k.sizeUnknown) ? t('files.unmeasured') : size;
  if (one.value) return one.value.dir ? `${size}, ${st.dirs ? t('props.inDirs', { files: plural(st.files, 'ui.files'), dirs: plural(st.dirs, 'props.dirsWord') }) : plural(st.files, 'ui.files')}` : size;
  return `${size}, ${st.dirs ? t('props.andDirs', { files: plural(st.files, 'ui.files'), dirs: plural(st.dirs, 'props.dirsPlain') }) : plural(st.files, 'ui.files')}`;
});
const title = computed(() => (one.value ? (st.self ? st.label : one.value.name) : plural(st.nodes.length, 'ui.objects')));
const newest = computed(() => Math.max(...st.nodes.map((k) => k.time)));

const WHO = [0, 1, 2], BITS = [4, 2, 1];
const FLAGS = ['readOnly', 'hidden', 'archive'] as const;
const on = (who: number, bit: number) => !!(Number(st.mode[1 + who]) & bit);
function flip(who: number, bit: number) {
  const digits = [...st.mode];
  digits[1 + who] = String(Number(digits[1 + who]) ^ bit);
  st.mode = digits.join('');
}
const oct = ref(st.mode.slice(1));
watch(() => st.mode, (m) => { if (oct.value.padStart(4, '0') !== m) oct.value = m.slice(1); });
watch(oct, (v) => { const s = v.trim(); if (/^[0-7]{3,4}$/.test(s)) st.mode = s.padStart(4, '0'); });
const symbolic = computed(() => (one.value?.dir ? 'd' : '-') + [...st.mode.slice(-3)].map((c) => {
  const v = Number(c);
  return (v & 4 ? 'r' : '-') + (v & 2 ? 'w' : '-') + (v & 1 ? 'x' : '-');
}).join(''));
</script>

<template>
  <div class="bk-phead"><GlyphTile v-if="one" :glyph="glyphComponent(glyphOf(one))" ground :quiet="!one.dir" /><GlyphTile v-else :glyph="glyphComponent('IconFiles')" ground quiet /><span class="bk-name">{{ title }}</span></div>
  <div class="bk-grid2 bk-top" @keydown.enter="pressPrimary">
    <SubCard :title="t('props.general')" class="bk-form">
      <template v-if="one">
        <FieldRow :label="t('props.kind')"><span class="bk-val">{{ typeOf(one) }}</span></FieldRow>
        <FieldRow :label="t('props.place')"><span class="bk-val bk-pathv"><span class="bk-path mono" :title="st.paths[0]"><bdi>{{ st.paths[0] }}</bdi></span><CopyBtn :text="st.paths[0]!" :tip="t('props.copyPath')" /></span></FieldRow>
        <FieldRow v-if="one.link" :label="t('props.pointsTo')"><span class="bk-val bk-pathv"><span class="bk-path mono"><bdi>{{ one.link }}</bdi></span><CopyBtn :text="one.link" :tip="t('props.copyTarget')" /></span></FieldRow>
        <FieldRow :label="t('props.size')"><span class="bk-val glim-num" :data-tip="bytes">{{ sizeText }}</span></FieldRow>
        <FieldRow :label="t('props.changed')"><span class="bk-val glim-num">{{ fmtTime(one.time) }}</span></FieldRow>
        <FieldRow v-if="one.btime" :label="t('props.created')"><span class="bk-val glim-num">{{ fmtTime(one.btime) }}</span></FieldRow>
      </template>
      <template v-else>
        <FieldRow :label="t('props.selection')"><span class="bk-val">{{ plural(st.nodes.length, 'ui.objects') }}</span></FieldRow>
        <FieldRow :label="t('props.place')"><span class="bk-val bk-pathv"><span class="bk-path mono"><bdi>{{ st.dir }}</bdi></span><CopyBtn :text="st.paths.join('\n')" :tip="t('props.copyPaths')" /></span></FieldRow>
        <FieldRow :label="t('props.size')"><span class="bk-val glim-num" :data-tip="bytes">{{ sizeText }}</span></FieldRow>
        <FieldRow :label="t('props.lastChanged')"><span class="bk-val glim-num">{{ fmtTime(newest) }}</span></FieldRow>
      </template>
    </SubCard>
    <SubCard v-if="st.win" :title="t('props.owning')" class="bk-form">
      <FieldRow :label="t('props.owner')"><span id="r-owner" class="bk-val">{{ st.owner }}</span></FieldRow>
      <FieldRow :label="t('props.inside')" :hint="anyDir ? t('props.insideFlagsTip') : t('props.onlyDirs')"><Toggle id="r-rec" v-model="st.rec" :disabled="!anyDir || st.ro" /></FieldRow>
    </SubCard>
    <SubCard v-else :title="t('props.owning')" class="bk-form">
      <FieldRow :label="t('props.owner')" :hint="t('props.ownerTip')"><span class="bk-idf"><Select id="r-owner" v-model="st.owner" :trigger-attrs="{ 'data-val': st.owner }" :disabled="st.ro" :options="selectOptions(owners)" /><Pill id="r-uid" class="glim-num">{{ idOf(st.uids, st.owner, 'UID') }}</Pill></span></FieldRow>
      <FieldRow :label="t('props.group')"><span class="bk-idf"><Select id="r-group" v-model="st.group" :trigger-attrs="{ 'data-val': st.group }" :disabled="st.ro" :options="selectOptions(groups)" /><Pill id="r-gid" class="glim-num">{{ idOf(st.gids, st.group, 'GID') }}</Pill></span></FieldRow>
      <FieldRow :label="t('props.inside')" :hint="anyDir ? t('props.insideTip') : t('props.onlyDirs')"><Toggle id="r-rec" v-model="st.rec" :disabled="!anyDir || st.ro" /></FieldRow>
    </SubCard>
  </div>
  <template v-if="one && !st.ro"><Render v-for="(well, n) in extend.propsWells" :key="n" :node="well(st.paths[0]!)" /></template>
  <SubCard v-if="st.win" class="bk-form"><div class="bk-tools"><span class="glim-eyebrow">{{ t('props.attributes') }}</span></div>
    <div class="bk-flags">
      <Checkbox v-for="f in FLAGS" :key="f" v-model="st.flags[f]" :data-bk-flag="f" :label="t(`props.flags.${f}`)" :disabled="st.ro" />
    </div>
  </SubCard>
  <SubCard v-else class="bk-form"><div class="bk-tools"><span class="glim-eyebrow">{{ t('props.rights') }}</span><Pill v-if="st.modes.length > 1" :data-tip="t('props.mixedTip')" tone="warn">{{ t('props.mixed') }}</Pill></div>
    <div class="bk-perms">
      <span></span><span v-for="bit in BITS" :key="bit" class="bk-ph">{{ t(`props.bits.${bit}`) }}</span>
      <template v-for="who in WHO" :key="who">
        <span class="bk-pl">{{ t(`props.who.${who}`) }}</span>
        <span v-for="bit in BITS" :key="bit" class="bk-pc"><Checkbox slim :model-value="on(who, bit)" :data-bk-perm="`${who}:${bit}`" :label="`${t(`props.who.${who}`)}: ${t(`props.bits.${bit}`)}`" :disabled="st.ro" @update:model-value="flip(who, bit)" /></span>
      </template>
    </div>
    <FieldRow :label="t('props.asNumber')" :hint="t('props.asNumberTip')"><span class="bk-idf"><Pill id="r-sym" class="mono">{{ symbolic }}</Pill><TextField id="r-oct" v-model="oct" class="glim-num" maxlength="4" inputmode="numeric" :disabled="st.ro" width="short" mono /></span></FieldRow>
  </SubCard>
</template>
