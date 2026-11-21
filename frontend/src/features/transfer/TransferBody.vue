<script setup lang="ts">
// The copy, move and hard link window: one sentence for what goes where, and the target to change. Names that exist
// in the target are asked about when the job meets them.
import { pathField } from '@/ui/picker';
import { Checkbox, FieldRow, PathField, Pill } from '@glimstone/vue';
import { computed } from 'vue';
import type { Preflight, Rule } from '@/api/ops';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { plural, t } from '@/i18n';
import { ensureDir, placeName, sizeOf } from './paths';
import SpacePanel from './SpacePanel.vue';

export interface TransferState {
  kind: 'copy' | 'move' | 'link';
  items: Entry[];
  from: string;
  dest: string;
  rule: Rule;
  verify: boolean;
  pf: Preflight | null;
}
const props = defineProps<{ st: TransferState }>();

const GLYPH = { copy: 'IconCopy', move: 'IconMove', link: 'IconLink' } as const;
const total = computed(() => props.st.items.reduce((a, k) => a + sizeOf(k), 0));
const what = computed(() => (props.st.items.length === 1 ? props.st.items[0]!.name : plural(props.st.items.length, 'transfer.objects')));
const clean = (p: string) => p.trim().replace(/(.)\/$/, '$1') || '/';
const same = computed(() => props.st.pf?.same ?? []);
</script>

<template>
  <div class="xsheet">
    <span class="xsheet-tile"><G :n="GLYPH[st.kind]" /></span>
    <span class="xsheet-say">
      <b>{{ t(`transfer.ask.${st.kind}`) }}</b>
      <span class="glim-num">{{ t('transfer.into', { what, size: fmtSize(total) || '0 B', to: placeName(clean(st.dest)) }) }}</span>
    </span>
  </div>
  <p v-if="st.kind === 'link'" class="xnote">{{ t('transfer.linkTip') }}</p>
  <FieldRow :label="t('transfer.to')"><PathField v-model="st.dest" v-bind="pathField('dest')" /></FieldRow>
  <SpacePanel v-if="st.pf?.unraid?.space" :space="st.pf.unraid.space" :dest="clean(st.dest)" @dest="(p) => ensureDir(p).then(() => { st.dest = p; })" />
  <FieldRow v-if="same.length" :label="t('transfer.sameSkip')" :hint="t('transfer.sameSkipTip')">
    <span id="xsame" class="xsame"><Pill v-for="n in same.slice(0, 3)" :key="n" tone="state">{{ n }}</Pill><Pill v-if="same.length > 3">{{ plural(same.length - 3, 'transfer.more') }}</Pill></span>
  </FieldRow>
  <Checkbox v-if="st.kind !== 'link'" v-model="st.verify" class="xverify" :label="t('transfer.verify')" :title="t('transfer.verifyTip')" />
</template>
