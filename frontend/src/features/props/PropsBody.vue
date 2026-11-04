<script setup lang="ts">
// Eigenschaften und Rechte: what the selection is, who owns it, and the nine rights as boxes and as a number.
import { computed, ref, watch } from 'vue';
import type { Item } from '@/api/client';
import type { UnraidWhere } from '@/api/tools';
import { fmtSize, fmtTime } from '@/core/format';
import { ext, G, kindOf } from '@/core/glyphs';
import { diskName } from '@/features/jobs/label';
import { locale, t } from '@/i18n';
import CopyBtn from '@/ui/CopyBtn.vue';
import Dropdown from '@/ui/Dropdown.vue';
import Frow from '@/ui/Frow.vue';
import { plural, pressPrimary } from '@/ui/kit';
import Sw from '@/ui/Sw.vue';
import Tile from '@/ui/Tile.vue';

export interface PropsState {
  nodes: Item[]; paths: string[]; dir: string; self: boolean; ro: boolean; uids: Record<string, number>; gids: Record<string, number>;
  modes: string[]; owners: string[]; groups: string[]; mode: string; owner: string; group: string; rec: boolean;
  size: number; files: number; dirs: number; counted: boolean; label: string; where: UnraidWhere | null;
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
const bytes = computed(() => t('props.bytes', { n: Math.round(st.size).toLocaleString(locale()) }));
const sizeText = computed(() => {
  const size = fmtSize(st.size) || '0 B';
  if (!st.counted) return size;
  if (one.value) return one.value.dir ? `${size}, ${st.dirs ? t('props.inDirs', { files: plural(st.files, 'ui.files'), dirs: plural(st.dirs, 'props.dirsWord') }) : plural(st.files, 'ui.files')}` : size;
  return `${size}, ${st.dirs ? t('props.andDirs', { files: plural(st.files, 'ui.files'), dirs: plural(st.dirs, 'props.dirsPlain') }) : plural(st.files, 'ui.files')}`;
});
const title = computed(() => (one.value ? (st.self ? st.label : one.value.name) : plural(st.nodes.length, 'ui.objects')));
const newest = computed(() => Math.max(...st.nodes.map((k) => k.time)));

const WHO = [0, 1, 2], BITS = [4, 2, 1];
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
  <div class="bk-phead"><Tile v-if="one" :k="one" /><Tile v-else g="IconFiles" /><span class="bk-name">{{ title }}</span></div>
  <div class="bk-grid2 bk-top" @keydown.enter="pressPrimary">
    <div class="well bk-form"><div class="eyebrow">{{ t('props.general') }}</div>
      <template v-if="one">
        <Frow :label="t('props.kind')"><span class="bk-val">{{ typeOf(one) }}</span></Frow>
        <Frow :label="t('props.place')"><span class="bk-val bk-pathv"><span class="bk-path mono" :title="st.paths[0]"><bdi>{{ st.paths[0] }}</bdi></span><CopyBtn :text="st.paths[0]!" :tip="t('props.copyPath')" /></span></Frow>
        <Frow v-if="one.link" :label="t('props.pointsTo')"><span class="bk-val bk-pathv"><span class="bk-path mono"><bdi>{{ one.link }}</bdi></span><CopyBtn :text="one.link" :tip="t('props.copyTarget')" /></span></Frow>
        <Frow :label="t('props.size')"><span class="bk-val glim-num" :data-tip="bytes">{{ sizeText }}</span></Frow>
        <Frow :label="t('props.changed')"><span class="bk-val glim-num">{{ fmtTime(one.time) }}</span></Frow>
        <Frow v-if="one.btime" :label="t('props.created')"><span class="bk-val glim-num">{{ fmtTime(one.btime) }}</span></Frow>
      </template>
      <template v-else>
        <Frow :label="t('props.selection')"><span class="bk-val">{{ plural(st.nodes.length, 'ui.objects') }}</span></Frow>
        <Frow :label="t('props.place')"><span class="bk-val bk-pathv"><span class="bk-path mono"><bdi>{{ st.dir }}</bdi></span><CopyBtn :text="st.paths.join('\n')" :tip="t('props.copyPaths')" /></span></Frow>
        <Frow :label="t('props.size')"><span class="bk-val glim-num" :data-tip="bytes">{{ sizeText }}</span></Frow>
        <Frow :label="t('props.lastChanged')"><span class="bk-val glim-num">{{ fmtTime(newest) }}</span></Frow>
      </template>
    </div>
    <div class="well bk-form"><div class="eyebrow">{{ t('props.owning') }}</div>
      <Frow :label="t('props.owner')" :tip="t('props.ownerTip')"><span class="bk-idf"><Dropdown id="r-owner" v-model="st.owner" :options="owners" :disabled="st.ro" /><span id="r-uid" class="pill glim-num">{{ idOf(st.uids, st.owner, 'UID') }}</span></span></Frow>
      <Frow :label="t('props.group')"><span class="bk-idf"><Dropdown id="r-group" v-model="st.group" :options="groups" :disabled="st.ro" /><span id="r-gid" class="pill glim-num">{{ idOf(st.gids, st.group, 'GID') }}</span></span></Frow>
      <Frow :label="t('props.inside')" :tip="anyDir ? t('props.insideTip') : t('props.onlyDirs')"><Sw id="r-rec" v-model="st.rec" :disabled="!anyDir || st.ro" /></Frow>
    </div>
  </div>
  <div v-if="st.where" class="well bk-form u-where"><div class="eyebrow">{{ t('props.storage') }}</div>
    <Frow :label="t('props.liesOn')" :tip="t('props.liesOnTip')"><span class="bk-val u-pills">
      <span v-for="d in st.where.on" :key="d.device" class="pill glim-num" :class="{ state: d.device === 'cache' }">{{ diskName(d.device) }}<template v-if="st.where.files > 1">, {{ fmtSize(d.size) || '0 B' }}</template></span>
      <span v-if="!st.where.on.length" class="muted">{{ t('props.nothing') }}</span>
    </span></Frow>
    <Frow v-if="st.where.links" :label="t('props.hardLinks')" :tip="t('props.hardLinksTip')">
      <span v-if="st.where.links.paths.length" class="bk-val u-links"><span v-for="p in st.where.links.paths" :key="p" class="bk-pathv"><span class="bk-path mono" :title="p"><bdi>{{ p }}</bdi></span><CopyBtn :text="p" :tip="t('props.copyPath')" /></span></span>
      <span v-else class="bk-val">{{ t('props.onlyThis') }}</span>
    </Frow>
  </div>
  <div class="well bk-form"><div class="bk-tools"><span class="eyebrow">{{ t('props.rights') }}</span><span v-if="st.modes.length > 1" class="pill warn" :data-tip="t('props.mixedTip')">{{ t('props.mixed') }}</span></div>
    <div class="bk-perms">
      <span></span><span v-for="bit in BITS" :key="bit" class="bk-ph">{{ t(`props.bits.${bit}`) }}</span>
      <template v-for="who in WHO" :key="who">
        <span class="bk-pl">{{ t(`props.who.${who}`) }}</span>
        <span v-for="bit in BITS" :key="bit" class="bk-pc"><button class="col-check" type="button" role="checkbox" :aria-checked="on(who, bit)" :data-bk-perm="`${who}:${bit}`" :aria-label="`${t(`props.who.${who}`)}: ${t(`props.bits.${bit}`)}`" :disabled="st.ro" @click="flip(who, bit)"><span class="cbox" :class="{ on: on(who, bit) }"><G n="IconCheck" /></span></button></span>
      </template>
    </div>
    <Frow :label="t('props.asNumber')" :tip="t('props.asNumberTip')"><span class="bk-idf"><span id="r-sym" class="pill mono">{{ symbolic }}</span><input id="r-oct" v-model="oct" class="field s mono glim-num" maxlength="4" inputmode="numeric" :disabled="st.ro"></span></Frow>
  </div>
</template>
