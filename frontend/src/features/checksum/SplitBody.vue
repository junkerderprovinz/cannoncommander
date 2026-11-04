<script setup lang="ts">
// Datei teilen: how big each part is, a sum file beside them, and where the parts land.
import { computed, watchEffect } from 'vue';
import { fmtSize } from '@/core/format';
import type { Entry } from '@/files/entries';
import { join } from '@/files/paths';
import { t } from '@/i18n';
import BkPath from '@/ui/BkPath.vue';
import Frow from '@/ui/Frow.vue';
import Seg from '@/ui/Seg.vue';
import Sw from '@/ui/Sw.vue';
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
  <div class="well"><div class="bk-filehead"><Tile :k="st.k" /><span class="bk-name">{{ st.k.name }}</span><span class="pill glim-num">{{ fmtSize(st.k.size) }}</span></div></div>
  <div class="well bk-form"><div class="eyebrow">{{ t('checksum.parts') }}</div>
    <Frow :label="t('checksum.partSize')" :tip="t('checksum.partSizeTip')"><Seg id="t-size" v-model="st.size" :options="sizes" /></Frow>
    <Frow :label="t('checksum.ownSize')"><span class="bk-range"><input id="t-custom" v-model="st.custom" class="field s glim-num" inputmode="numeric" :disabled="st.size !== 'custom'"><span class="muted">MB</span></span></Frow>
    <Frow :label="t('checksum.addSum')" :tip="t('checksum.addSumTip')"><Sw id="t-sum" v-model="st.sum" /></Frow>
  </div>
  <div class="well bk-form"><div class="eyebrow">{{ t('checksum.target') }}</div>
    <Frow :label="t('checksum.folder')"><BkPath id="t-dest" v-model="st.dest" /></Frow>
    <Frow :label="t('checksum.yields')">
      <span id="t-final" class="bk-finalw">
        <span v-if="ok === false" class="pill fault">{{ t('ui.noFolder') }}</span>
        <span v-else-if="st.count < 2" class="pill warn">{{ t('checksum.smaller') }}</span>
        <template v-else><span class="pill glim-num">{{ t('checksum.partsCount', { n: st.count }) }}</span><span class="bk-path mono bk-final" :title="join(clean(st.dest), names[0]!)"><bdi>{{ t('checksum.range', { first: names[0]!, last: names.at(-1)!.split('.').pop()! }) }}</bdi></span></template>
      </span>
    </Frow>
  </div>
</template>
