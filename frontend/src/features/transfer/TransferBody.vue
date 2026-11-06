<script setup lang="ts">
// The copy and move window: where from and where to, how many and how much, and what to do with names that exist.
import { pathField } from '@/ui/picker';
import { segItems, selectOptions } from '@/ui/kit';
import { Select } from '@glimstone/vue/Select';
import { FieldRow, FieldRowGroup, PathField, Pill, Selector, SubCard, Toggle } from '@glimstone/vue';
import { computed, watch } from 'vue';
import type { Preflight, Rule } from '@/api/ops';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { plural, t } from '@/i18n';
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
const same = computed(() => props.st.pf?.same ?? []);
watch(linkable, (ok) => { if (!ok) props.st.way = props.st.kind; });
</script>

<template>
  <div class="xroute">
    <div class="xend"><span class="xic"><G n="IconFolderOpen" /></span><b>{{ placeName(st.from) }}</b></div>
    <div class="xmid">
      <span class="xsum glim-num">{{ plural(st.items.length, 'transfer.objects') }} · {{ fmtSize(total) || '0 B' }}</span>
      <span class="xtrack"><span class="xline"></span><span class="xarrow"><G :n="st.kind === 'copy' ? 'IconCopy' : 'IconMove'" /></span></span>
    </div>
    <div class="xend"><span class="xic"><G n="IconFolder" /></span><b id="xdest">{{ placeName(clean(st.dest)) }}</b></div>
  </div>
  <ItemChips :items="st.items" />
  <SpacePanel v-if="st.pf?.unraid?.space" :space="st.pf.unraid.space" :dest="clean(st.dest)" @dest="(p) => ensureDir(p).then(() => { st.dest = p; })" />
  <SubCard>
    <FieldRowGroup>
      <FieldRow :label="t('transfer.to')"><PathField v-model="st.dest" v-bind="pathField('dest')" /></FieldRow>
      <FieldRow v-if="same.length" :label="t('transfer.sameSkip')" :hint="t('transfer.sameSkipTip')">
        <span id="xsame" class="xsame"><Pill v-for="n in same.slice(0, 3)" :key="n" tone="state">{{ n }}</Pill><Pill v-if="same.length > 3">{{ plural(same.length - 3, 'transfer.more') }}</Pill></span>
      </FieldRow>
      <FieldRow v-if="linkable" :label="t('transfer.way')" :hint="t('transfer.linkTip')"><Selector id="how" v-model="st.way" :items="segItems(ways)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
      <FieldRow :label="t('transfer.clash')" :hint="t('transfer.clashTip')"><Select id="clash" v-model="st.rule" :trigger-attrs="{ 'data-val': st.rule }" :options="selectOptions(RULES())" rows="menu" /></FieldRow>
    </FieldRowGroup>
    <details class="xmore"><summary>{{ t('transfer.moreOptions') }}</summary>
      <FieldRow :label="t('transfer.keepAttrs')" :hint="t('transfer.keepAttrsTip')"><Toggle v-model="st.keepAttrs" /></FieldRow>
      <FieldRow :label="t('transfer.verify')" :hint="t('transfer.verifyTip')"><Toggle v-model="st.verify" /></FieldRow>
    </details>
  </SubCard>
</template>
