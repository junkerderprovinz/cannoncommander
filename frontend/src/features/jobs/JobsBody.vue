<script setup lang="ts">
// The jobs window: every job with its progress, the open ones in queue order and movable by arrow, drag or Alt+arrow.
import { segItems } from '@/ui/kit';
import { EmptyState, FieldRow, IconButton, Pill, ProgressBar, Selector, SubCard } from '@glimstone/vue';
import { computed, ref } from 'vue';
import type { Job } from '@/api/ops';
import { G, glyphOf, glyphComponent } from '@/core/glyphs';
import { prefs, setPref } from '@/core/prefs';
import { t } from '@/i18n';
import { isOpen, useJobs } from '@/stores/jobs';
import { useReorder } from '@/ui/reorder';
import { factsOf, jobLabel, pctOf } from './label';
import { answer, limitMenu } from './steps';

const jobs = useJobs();
const mode = computed({ get: () => (prefs.jobMode === 'parallel' ? 'parallel' : 'serial'), set: (v) => setPref('jobMode', v) });
const MODES = [['serial', t('jobs.serial')], ['parallel', t('jobs.parallel')]] as const;

// While a row is carried the others make room for it; the server hears of the new place on the drop.
const list = ref<HTMLElement | null>(null);
const drag = useReorder({
  ids: () => jobs.running.map((j) => j.id), list, attr: 'data-movable', arm: 'move',
  onReorder: (ids, id) => {
    const j = jobs.running.find((x) => x.id === id);
    if (j) jobs.move(j, ids.indexOf(id));
  },
});
const rows = computed(() => {
  const open = drag.order.value.map((id) => jobs.running.find((j) => j.id === id)!);
  return [...open, ...jobs.list.filter((j) => !isOpen(j))];
});
const waitingPlace = (j: Job) => jobs.running.filter((x) => x.state === 'queued').indexOf(j);
const many = computed(() => jobs.running.length > 1);
const step = (j: Job, d: number) => jobs.move(j, jobs.running.indexOf(j) + d);

function act(j: Job, what: string, e: MouseEvent) {
  ({
    pause: () => jobs.pause(j), cancel: () => jobs.cancel(j), retry: () => jobs.retry(j), answer: () => answer(j),
    up: () => step(j, -1), down: () => step(j, 1), limit: () => limitMenu(j, e.currentTarget as HTMLElement),
  } as Record<string, () => unknown>)[what]!();
}
function keys(j: Job, e: KeyboardEvent) {
  if (!isOpen(j) || !e.altKey || !['ArrowUp', 'ArrowDown'].includes(e.key)) return;
  e.preventDefault();
  step(j, e.key === 'ArrowUp' ? -1 : 1);
}
// A press on one of the row's buttons belongs to the button.
function press(j: Job, e: PointerEvent) {
  if (isOpen(j) && !(e.target as Element).closest('button')) drag.press(e, j.id);
}
</script>

<template>
  <SubCard><FieldRow :label="t('jobs.modeLabel')" :hint="t('jobs.modeTip')"><Selector id="jobs-mode" v-model="mode" :items="segItems(MODES)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow></SubCard>
  <SubCard id="jobs-body">
    <div v-if="rows.length" ref="list" class="job-list glim-drag-rows">
      <div v-for="j in rows" :key="j.id" class="job" :class="[drag.look(j.id), { open: isOpen(j) }]" :data-job="j.id" :data-movable="isOpen(j) ? j.id : undefined" :tabindex="isOpen(j) ? 0 : undefined"
        :aria-label="t('jobs.aria', { label: jobLabel(j), pct: Math.round(pctOf(j)), state: t(`jobs.state.${j.state}`) })"
        @keydown="keys(j, $event)" @pointerdown="press(j, $event)">
        <span class="job-grip" aria-hidden="true"><G v-if="isOpen(j)" n="IconGrip" /></span>
        <div class="job-text">
          <div class="job-label">{{ jobLabel(j) }}</div>
          <div class="sub glim-num">{{ factsOf({ ...j, place: j.place ?? waitingPlace(j) + 1 }) }}</div>
          <div v-if="j.state === 'running' && j.totalFiles > 1 && j.current" class="job-file mono" :data-tip="t('jobs.inWork')"><G :n="glyphOf({ name: j.current })" /><span>{{ j.current }}</span></div>
        </div>
        <ProgressBar class="bar" :value="pctOf(j) / 100" :label="jobLabel(j)" :paused="j.state === 'paused'" :failed="j.state === 'failed'" /><Pill class="glim-num">{{ Math.round(pctOf(j)) }} %</Pill>
        <span class="job-acts">
          <template v-if="isOpen(j)">
            <IconButton v-if="j.kind !== 'upload'" :pressed="!!j.cap" :tip="t('jobs.limit')" :glyph="glyphComponent('IconSliders')" data-jact="limit" @click="act(j, 'limit', $event)" />
            <template v-if="many">
              <IconButton :tip="t('jobs.earlier')" :glyph="glyphComponent('IconMoveUp')" data-jact="up" @click="act(j, 'up', $event)" />
              <IconButton :tip="t('jobs.later')" :glyph="glyphComponent('IconMoveDown')" data-jact="down" @click="act(j, 'down', $event)" />
            </template>
            <IconButton v-if="j.state === 'asking'" :tip="t('jobs.answer')" :glyph="glyphComponent('IconHelp')" data-jact="answer" @click="act(j, 'answer', $event)" />
            <IconButton v-else :tip="j.state === 'paused' ? t('jobs.resume') : t('jobs.pause')" :glyph="glyphComponent(j.state === 'paused' ? 'IconPlay' : 'IconPause')" data-jact="pause" @click="act(j, 'pause', $event)" />
            <IconButton :tip="t('jobs.cancel')" :glyph="glyphComponent('IconStop')" data-jact="cancel" @click="act(j, 'cancel', $event)" />
          </template>
          <IconButton v-else-if="j.state === 'failed' || j.state === 'cancelled'" :tip="t('jobs.retry')" :glyph="glyphComponent('IconRetry')" data-jact="retry" @click="act(j, 'retry', $event)" />
        </span>
      </div>
    </div>
    <EmptyState v-else nested :glyph="glyphComponent('IconQueued')" :title="t('jobs.empty')" />
  </SubCard>
</template>
