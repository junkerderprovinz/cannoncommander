<script setup lang="ts">
// Datei teilen: how big each part is, a sum file beside them, and where the parts land.
import { pathField } from '@/ui/picker';
import { segItems } from '@/ui/kit';
import { FieldRow, PathField, Pill, Selector, TextField, Toggle } from '@glimstone/vue';
import { computed, watchEffect } from 'vue';
import { fmtSize } from '@/core/format';
import type { Entry } from '@/files/entries';
import { join } from '@/files/paths';
import { t } from '@/i18n';
import Tile from '@/ui/Tile.vue';
import { clean, folderState } from '@/ui/tools';
import { partSize } from './sums';

export interface SplitState { k: Entry; size: string; custom: string; sum: boolean; dest: string; count: number; ok: boolean }
const props = defineProps<{ st: SplitState }>();
const sizes = computed(() => ['100', '1024', '4096', 'custom'].map((s) => [s, t(`checksum.sizes.${s}`)] as const));
const ok = folderState(() => props.st.dest);
const names = computed(() => {
  const n = props.st.count, pad = Math.max(3, String(n).length);
  return Array.from({ length: n }, (_, i) => `${props.st.k.name}.${String(i + 1).padStart(pad, '0')}`);
});
watchEffect(() => {
  const part = partSize(props.st);
  props.st.count = part > 0 ? Math.ceil(props.st.k.size / part) : 0;
  props.st.ok = !!ok.value;
});
</script>

<template>
  <div class="well glim-well"><div class="bk-filehead"><Tile :k="st.k" /><span class="bk-name">{{ st.k.name }}</span><Pill class="glim-num">{{ fmtSize(st.k.size) }}</Pill></div></div>
  <div class="well glim-well bk-form"><div class="eyebrow">{{ t('checksum.parts') }}</div>
    <FieldRow :label="t('checksum.partSize')" :hint="t('checksum.partSizeTip')"><Selector id="t-size" v-model="st.size" :items="segItems(sizes)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="t('checksum.ownSize')"><span class="bk-range"><TextField id="t-custom" v-model="st.custom" class="glim-num" inputmode="numeric" :disabled="st.size !== 'custom'" width="short" /><span class="muted">MB</span></span></FieldRow>
    <FieldRow :label="t('checksum.addSum')" :hint="t('checksum.addSumTip')"><Toggle id="t-sum" v-model="st.sum" /></FieldRow>
  </div>
  <div class="well glim-well bk-form"><div class="eyebrow">{{ t('checksum.target') }}</div>
    <FieldRow :label="t('checksum.folder')"><PathField v-model="st.dest" v-bind="pathField('t-dest')" /></FieldRow>
    <FieldRow :label="t('checksum.yields')">
      <span id="t-final" class="bk-finalw">
        <Pill v-if="ok === false" tone="fail">{{ t('ui.noFolder') }}</Pill>
        <Pill v-else-if="st.count < 2" tone="warn">{{ t('checksum.smaller') }}</Pill>
        <template v-else><Pill class="glim-num">{{ t('checksum.partsCount', { n: st.count }) }}</Pill><span class="bk-path mono bk-final" :title="join(clean(st.dest), names[0]!)"><bdi>{{ t('checksum.range', { first: names[0]!, last: names.at(-1)!.split('.').pop()! }) }}</bdi></span></template>
      </span>
    </FieldRow>
  </div>
</template>
