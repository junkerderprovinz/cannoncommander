<script setup lang="ts">
// Ordner bündeln, after the unbalanced plugin: how a user share folder spreads over the disks, and what gathering it
// onto one disk or emptying one disk of it would move.
import { pathField } from '@/ui/picker';
import { segItems } from '@/ui/kit';
import { EmptyState, FieldRow, FieldRowGroup, Meter, PathField, Pill, Selector, SubCard } from '@glimstone/vue';
import { computed } from 'vue';
import type { GatherPlan } from '@/api/ops';
import { fmtSize } from '@/core/format';
import { G, glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { diskName } from '@/features/jobs/label';

export interface GatherState { path: string; mode: 'gather' | 'scatter'; disk: string; plan: GatherPlan | null; valid: boolean; text: string }
const props = defineProps<{ st: GatherState }>();
const MODES = [['gather', t('gather.gather')], ['scatter', t('gather.scatter')]] as const;

const files = (n: number) => (n === 1 ? t('gather.one') : t('gather.many', { n }));
const devices = computed(() => (props.st.plan?.devices ?? []).filter((d) => d.device !== 'cache' || d.files));
const total = computed(() => devices.value.reduce((a, d) => a + d.now, 0) || 1);
const options = computed(() => (props.st.plan?.options ?? []).map((o) => [o.disk, props.st.mode === 'gather' ? t('gather.targetOption', { disk: diskName(o.disk), size: fmtSize(o.move) || '0 B' }) : diskName(o.disk)] as const));
</script>

<template>
  <SubCard class="bk-form">
    <FieldRowGroup>
      <FieldRow :label="t('gather.folder')" :hint="t('gather.folderTip')"><PathField v-model="st.path" v-bind="pathField('g-dir')" /></FieldRow>
      <FieldRow :label="t('gather.task')" :hint="t('gather.taskTip')"><Selector id="g-mode" v-model="st.mode" :items="segItems(MODES)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    </FieldRowGroup>
    <div id="g-pick">
      <FieldRow v-if="st.valid && options.length" :label="st.mode === 'gather' ? t('gather.target') : t('gather.source')" :hint="st.mode === 'gather' ? t('gather.targetTip') : t('gather.sourceTip')">
        <Selector :id="st.mode === 'gather' ? 'g-target' : 'g-source'" v-model="st.disk" :items="segItems(options)" size="sm" activation="manual" inline button-height pairs-on-phone />
      </FieldRow>
    </div>
  </SubCard>
  <SubCard>
    <div class="bk-tools"><span id="g-sum" class="glim-eyebrow">{{ st.plan && st.valid ? t('gather.sum', { files: files(st.plan.files), size: fmtSize(st.plan.size) || '0 B' }) : t('gather.spread') }}</span></div>
    <div id="g-bars" class="u-dist">
      <EmptyState v-if="!st.valid" nested :glyph="glyphComponent('IconFolder')" :title="t('gather.pickUser')" />
      <template v-else>
        <span></span><span></span><span class="u-dh r">{{ t('gather.now') }}</span><span class="u-dh r">{{ t('gather.after') }}</span><span class="u-dh r">{{ t('gather.free') }}</span>
        <template v-for="d in devices" :key="d.device">
          <span class="u-dn" :class="{ on: d.device === st.disk }"><G :n="d.device === 'cache' ? 'IconBolt' : 'IconDatabase'" />{{ diskName(d.device) }}<G v-if="d.asleep" n="IconMoon" /></span>
          <Meter quiet :threshold="2" :value="d.now / total" :label="diskName(d.device)" />
          <Pill class="glim-num">{{ fmtSize(d.now) || '0 B' }}</Pill>
          <Pill class="glim-num" :tone="d.after !== d.now ? 'state' : 'neutral'">{{ fmtSize(d.after) || '0 B' }}</Pill>
          <span class="glim-num muted r">{{ fmtSize(d.free) }}</span>
        </template>
      </template>
    </div>
  </SubCard>
  <p id="g-plan" class="u-plan">{{ st.text }}</p>
</template>
