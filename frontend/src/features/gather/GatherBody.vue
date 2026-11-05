<script setup lang="ts">
// Ordner bündeln, after the unbalanced plugin: how a user share folder spreads over the disks, and what gathering it
// onto one disk or emptying one disk of it would move.
import { computed } from 'vue';
import type { GatherPlan } from '@/api/ops';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { diskName } from '@/features/jobs/label';
import Frow from '@/ui/Frow.vue';
import PathField from '@/ui/PathField.vue';
import Seg from '@/ui/Seg.vue';

export interface GatherState { path: string; mode: 'gather' | 'scatter'; disk: string; plan: GatherPlan | null; valid: boolean; text: string }
const props = defineProps<{ st: GatherState }>();
const MODES = [['gather', t('gather.gather')], ['scatter', t('gather.scatter')]] as const;

const files = (n: number) => (n === 1 ? t('gather.one') : t('gather.many', { n }));
const devices = computed(() => (props.st.plan?.devices ?? []).filter((d) => d.device !== 'cache' || d.files));
const total = computed(() => devices.value.reduce((a, d) => a + d.now, 0) || 1);
const options = computed(() => (props.st.plan?.options ?? []).map((o) => [o.disk, props.st.mode === 'gather' ? t('gather.targetOption', { disk: diskName(o.disk), size: fmtSize(o.move) || '0 B' }) : diskName(o.disk)] as const));
</script>

<template>
  <div class="well glim-well bk-form">
    <Frow :label="t('gather.folder')" :tip="t('gather.folderTip')"><PathField id="g-dir" v-model="st.path" /></Frow>
    <Frow :label="t('gather.task')" :tip="t('gather.taskTip')"><Seg id="g-mode" v-model="st.mode" :options="MODES" /></Frow>
    <div id="g-pick">
      <Frow v-if="st.valid && options.length" :label="st.mode === 'gather' ? t('gather.target') : t('gather.source')" :tip="st.mode === 'gather' ? t('gather.targetTip') : t('gather.sourceTip')">
        <Seg :id="st.mode === 'gather' ? 'g-target' : 'g-source'" v-model="st.disk" :options="options" />
      </Frow>
    </div>
  </div>
  <div class="well glim-well">
    <div class="bk-tools"><span id="g-sum" class="eyebrow">{{ st.plan && st.valid ? t('gather.sum', { files: files(st.plan.files), size: fmtSize(st.plan.size) || '0 B' }) : t('gather.spread') }}</span></div>
    <div id="g-bars" class="u-dist">
      <div v-if="!st.valid" class="empty"><G n="IconFolder" /><div>{{ t('gather.pickUser') }}</div></div>
      <template v-else>
        <span></span><span></span><span class="u-dh r">{{ t('gather.now') }}</span><span class="u-dh r">{{ t('gather.after') }}</span><span class="u-dh r">{{ t('gather.free') }}</span>
        <template v-for="d in devices" :key="d.device">
          <span class="u-dn" :class="{ on: d.device === st.disk }"><G :n="d.device === 'cache' ? 'IconBolt' : 'IconDatabase'" />{{ diskName(d.device) }}<G v-if="d.asleep" n="IconMoon" /></span>
          <span class="bar"><i :style="{ width: ((d.now / total) * 100).toFixed(1) + '%' }"></i></span>
          <span class="pill glim-num">{{ fmtSize(d.now) || '0 B' }}</span>
          <span class="pill glim-num" :class="{ state: d.after !== d.now }">{{ fmtSize(d.after) || '0 B' }}</span>
          <span class="glim-num muted r">{{ fmtSize(d.free) }}</span>
        </template>
      </template>
    </div>
  </div>
  <p id="g-plan" class="u-plan">{{ st.text }}</p>
</template>
