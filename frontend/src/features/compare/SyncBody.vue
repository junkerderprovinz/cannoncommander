<script setup lang="ts">
// The sync window: direction, how to compare and whether to mirror, and the plan as a dry run whose steps can each
// be switched off. The server plans; a step that would overwrite something newer comes switched off.
import { EmptyState, FieldRow, InfoBubble, Pill, Selector, SubCard, Toggle } from '@glimstone/vue';
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue';
import { type SyncDirection, type SyncStep, tools } from '@/api/tools';
import { run } from '@/core/actions';
import { fmtSize } from '@/core/format';
import { G, glyphComponent } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { split } from '@/files/paths';
import { t } from '@/i18n';
import { plural, segItems } from '@/ui/kit';
import Tile from '@/ui/Tile.vue';
import { startSync } from './sync';

const props = defineProps<{ left: string; right: string; win: WinEntry; close: () => void }>();
const o = reactive({ dir: 'right' as SyncDirection, by: 'time' as 'time' | 'content', mirror: false, recursive: true });
const steps = ref<SyncStep[]>([]);
const touched = reactive(new Map<string, boolean>());

const DIRS = computed(() => [['left', t('compare.toLeft'), 'IconBack'], ['both', t('compare.both'), 'IconSync'], ['right', t('compare.toRight'), 'IconForward']] as const);
const BY = computed(() => [['time', t('compare.byTime')], ['content', t('compare.byContent')]] as const);
const arrow = computed(() => (o.dir === 'left' ? 'IconBack' : o.dir === 'right' ? 'IconForward' : 'IconSync'));

const isOn = (s: SyncStep) => (touched.has(s.key) ? touched.get(s.key)! : !s.off);
const on = computed(() => steps.value.filter(isOn));
const count = computed(() => (steps.value.length
  ? t('compare.steps', { on: plural(on.value.length, 'compare.stepCount'), all: steps.value.length, size: fmtSize(on.value.filter((s) => s.act !== 'delete').reduce((a, s) => a + s.size, 0)) || '0 B' })
  : t('compare.preview')));

const nameOf = (s: SyncStep) => ({ name: split(s.rel).pop() || s.rel, dir: !!s.dir });
const actOf = (s: SyncStep) => (s.act === 'delete' ? t(s.to === 'R' ? 'compare.deleteRight' : 'compare.deleteLeft') : t(`compare.act.${s.act}`));
const glyphOf = (s: SyncStep) => (s.act === 'delete' ? 'IconTrash' : s.to === 'R' ? 'IconForward' : 'IconBack');

let asked = 0;
async function plan() {
  // Mirroring deletes on one side only, so it has no meaning in both directions.
  if (o.dir === 'both') o.mirror = false;
  const mine = ++asked;
  try {
    const got = await tools.syncPlan({ left: props.left, right: props.right, direction: o.dir, content: o.by === 'content', recursive: o.recursive, mirror: o.mirror, hidden: !!prefs.showHidden });
    if (mine === asked) steps.value = got;
  } catch (e) {
    if (mine === asked) { steps.value = []; toast((e as Error).message, 'warn'); }
  }
}
watch(o, plan, { immediate: true });

function apply() {
  const todo = on.value;
  if (!todo.length) return false;
  void startSync(props.left, props.right, todo);
  return true;
}
watch(() => on.value.length, (n) => {
  props.win.foot = [
    { start: t('ui.background') }, cancelBtn(),
    { label: t('arrowloop.title'), g: 'IconSchedules', run: () => { run('arrowLoop', props.left, props.right, { direction: o.dir, mirror: o.mirror }); return false; } },
    { label: t('compare.sync'), g: 'IconSync', primary: true, disabled: !n, run: apply },
  ];
}, { immediate: true });
onBeforeUnmount(() => { asked++; });
</script>

<template>
  <SubCard :title="t('compare.match')" class="bk-form">
    <div class="bk-sides"><span class="bk-side mono" :title="left"><bdi>{{ left }}</bdi></span><span id="y-arrow" class="bk-arrow"><G :n="arrow" /></span><span class="bk-side mono" :title="right"><bdi>{{ right }}</bdi></span></div>
    <div class="bk-rows">
      <FieldRow :label="t('compare.direction')"><Selector id="y-dir" v-model="o.dir" :items="segItems(DIRS)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
      <FieldRow :label="t('compare.by')" :hint="t('compare.byTip')"><Selector id="y-by" v-model="o.by" :items="segItems(BY)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
      <FieldRow :label="t('compare.recursive')"><Toggle id="y-rec" v-model="o.recursive" /></FieldRow>
      <FieldRow :label="t('compare.mirror')" :hint="t('compare.mirrorTip')"><Toggle id="y-mirror" v-model="o.mirror" :disabled="o.dir === 'both'" /></FieldRow>
    </div>
  </SubCard>
  <SubCard class="bk-fill">
    <div class="bk-tools"><span id="y-count" class="glim-eyebrow">{{ count }}</span><InfoBubble :tip="t('compare.previewTip')" /></div>
    <div id="y-list" class="bk-list">
      <template v-if="steps.length">
        <div v-for="(s, i) in steps" :key="s.key" class="bk-step" :class="{ skip: !isOn(s) }">
          <Toggle :model-value="isOn(s)" :data-bk-step="i" :label="s.rel" hide-label @update:model-value="(v: boolean) => touched.set(s.key, v)" />
          <Tile :k="nameOf(s)" />
          <span class="bk-name">{{ s.rel }}{{ s.dir ? '/' : '' }}</span>
          <Pill v-if="s.why" tone="warn">{{ t(`compare.why.${s.why}`) }}</Pill><span v-else></span>
          <span class="bk-arrow sm" :data-tip="t(s.to === 'R' ? 'compare.right' : 'compare.left')"><G :n="glyphOf(s)" /></span>
          <Pill>{{ actOf(s) }}</Pill>
          <Pill class="glim-num">{{ fmtSize(s.size) || '0 B' }}</Pill>
        </div>
      </template>
      <EmptyState v-else nested :glyph="glyphComponent('IconCheck')" :title="t('compare.nothing')" />
    </div>
  </SubCard>
</template>
