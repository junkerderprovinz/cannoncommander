<script setup lang="ts">
// The copy and move window: where from and where to, how many and how much, and what to do with names that exist.
import { computed, watch } from 'vue';
import type { Preflight, Rule } from '@/api/ops';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { t } from '@/i18n';
import Dropdown from '@/ui/Dropdown.vue';
import Frow from '@/ui/Frow.vue';
import PathField from '@/ui/PathField.vue';
import Seg from '@/ui/Seg.vue';
import Sw from '@/ui/Sw.vue';
import ItemChips from './ItemChips.vue';
import { ensureDir, placeName, sizeOf } from './paths';
import { RULES } from './rules';
import SpacePanel from './SpacePanel.vue';

export interface TransferState {
  kind: 'copy' | 'move';
  way: 'copy' | 'move' | 'link';
  items: Entry[];
  from: string;
  dest: string;
  rule: Rule;
  keepAttrs: boolean;
  verify: boolean;
  pf: Preflight | null;
}
const props = defineProps<{ st: TransferState }>();

const total = computed(() => props.st.items.reduce((a, k) => a + sizeOf(k), 0));
const verb = computed(() => t(`transfer.${props.st.kind}`));
const linkable = computed(() => props.st.kind === 'copy' && !!props.st.pf?.unraid?.linkable);
const ways = computed(() => [[props.st.kind, verb.value], ['link', t('transfer.linkWay')]] as const);
const clean = (p: string) => p.trim().replace(/(.)\/$/, '$1') || '/';
watch(linkable, (ok) => { if (!ok) props.st.way = props.st.kind; });
</script>

<template>
  <div class="xroute">
    <div class="xend"><span class="xic"><G n="IconFolderOpen" /></span><b>{{ placeName(st.from) }}</b></div>
    <div class="xmid">
      <span class="xsum glim-num">{{ st.items.length === 1 ? t('transfer.one') : t('transfer.many', { n: st.items.length }) }} · {{ fmtSize(total) || '0 B' }}</span>
      <span class="xtrack"><span class="xline"></span><span class="xarrow"><G :n="st.kind === 'copy' ? 'IconCopy' : 'IconMove'" /></span></span>
    </div>
    <div class="xend"><span class="xic"><G n="IconFolder" /></span><b id="xdest">{{ placeName(clean(st.dest)) }}</b></div>
  </div>
  <ItemChips :items="st.items" />
  <SpacePanel v-if="st.pf?.unraid?.space" :space="st.pf.unraid.space" :dest="clean(st.dest)" @dest="(p) => ensureDir(p).then(() => { st.dest = p; })" />
  <div class="well glim-well">
    <Frow :label="t('transfer.to')"><PathField id="dest" v-model="st.dest" /></Frow>
    <Frow v-if="linkable" :label="t('transfer.way')" :tip="t('transfer.linkTip')"><Seg id="how" v-model="st.way" :options="ways" /></Frow>
    <Frow :label="t('transfer.clash')" :tip="t('transfer.clashTip')"><Dropdown id="clash" v-model="st.rule" :options="RULES()" /></Frow>
    <details class="xmore"><summary>{{ t('transfer.moreOptions') }}</summary>
      <Frow :label="t('transfer.keepAttrs')" :tip="t('transfer.keepAttrsTip')"><Sw v-model="st.keepAttrs" /></Frow>
      <Frow :label="t('transfer.verify')" :tip="t('transfer.verifyTip')"><Sw v-model="st.verify" /></Frow>
    </details>
  </div>
</template>
