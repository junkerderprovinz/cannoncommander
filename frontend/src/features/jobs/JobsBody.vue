<script setup lang="ts">
// The jobs window: every job with its progress, the open ones in queue order and movable by arrow, drag or Alt+arrow.
import { computed, ref } from 'vue';
import type { Job } from '@/api/ops';
import { G, glyphOf } from '@/core/glyphs';
import { prefs, setPref } from '@/core/prefs';
import { t } from '@/i18n';
import { isOpen, useJobs } from '@/stores/jobs';
import Frow from '@/ui/Frow.vue';
import Seg from '@/ui/Seg.vue';
import { factsOf, jobLabel, pctOf } from './label';
import { answer, limitMenu } from './steps';

const jobs = useJobs();
const mode = computed({ get: () => (prefs.jobMode === 'parallel' ? 'parallel' : 'serial'), set: (v) => setPref('jobMode', v) });
const MODES = [['serial', t('jobs.serial')], ['parallel', t('jobs.parallel')]] as const;

// While a row is dragged the list shows where it would land; the server hears of it on the drop.
const dragging = ref<Job | null>(null);
const preview = ref<Job[] | null>(null);
const rows = computed(() => {
  const open = preview.value ?? jobs.running;
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
function dragStart(j: Job, e: DragEvent) {
  if (!isOpen(j)) return;
  dragging.value = j;
  preview.value = [...jobs.running];
  e.dataTransfer!.effectAllowed = 'move';
}
function dragOver(j: Job, e: DragEvent) {
  const d = dragging.value, list = preview.value;
  if (!d || !list || !isOpen(j) || j === d) return;
  e.preventDefault();
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const rest = list.filter((x) => x !== d);
  const at = rest.indexOf(j) + (e.clientY < r.top + r.height / 2 ? 0 : 1);
  rest.splice(at, 0, d);
  preview.value = rest;
}
function dragEnd() {
  const d = dragging.value, to = preview.value?.indexOf(d!) ?? -1;
  dragging.value = null;
  preview.value = null;
  if (d && to >= 0) jobs.move(d, to);
}
</script>

<template>
  <div class="well glim-well"><Frow :label="t('jobs.modeLabel')" :tip="t('jobs.modeTip')"><Seg id="jobs-mode" v-model="mode" :options="MODES" /></Frow></div>
  <div id="jobs-body" class="well glim-well">
    <template v-if="rows.length">
      <div v-for="j in rows" :key="j.id" class="job" :class="{ open: isOpen(j), dragging: dragging === j }" :data-job="j.id" :draggable="isOpen(j) ? 'true' : undefined" :tabindex="isOpen(j) ? 0 : undefined"
        :aria-label="t('jobs.aria', { label: jobLabel(j), pct: Math.round(pctOf(j)), state: t(`jobs.state.${j.state}`) })"
        @keydown="keys(j, $event)" @dragstart="dragStart(j, $event)" @dragover="dragOver(j, $event)" @dragend="dragEnd">
        <span class="job-grip" aria-hidden="true"><G v-if="isOpen(j)" n="IconGrip" /></span>
        <div class="job-text">
          <div class="job-label">{{ jobLabel(j) }}</div>
          <div class="sub glim-num">{{ factsOf({ ...j, place: j.place ?? waitingPlace(j) + 1 }) }}</div>
          <div v-if="j.state === 'running' && j.totalFiles > 1 && j.current" class="job-file mono" :data-tip="t('jobs.inWork')"><G :n="glyphOf({ name: j.current })" /><span>{{ j.current }}</span></div>
        </div>
        <div class="bar"><i :style="{ width: pctOf(j) + '%' }"></i></div><span class="pill glim-num">{{ Math.round(pctOf(j)) }} %</span>
        <span class="job-acts">
          <template v-if="isOpen(j)">
            <button v-if="j.kind !== 'upload'" class="btn icon" :class="{ on: j.cap }" data-jact="limit" :data-tip="t('jobs.limit')" :aria-label="t('jobs.limit')" @click="act(j, 'limit', $event)"><G n="IconSliders" /></button>
            <template v-if="many">
              <button class="btn icon" data-jact="up" :data-tip="t('jobs.earlier')" :aria-label="t('jobs.earlier')" @click="act(j, 'up', $event)"><G n="IconMoveUp" /></button>
              <button class="btn icon" data-jact="down" :data-tip="t('jobs.later')" :aria-label="t('jobs.later')" @click="act(j, 'down', $event)"><G n="IconMoveDown" /></button>
            </template>
            <button v-if="j.state === 'asking'" class="btn icon" data-jact="answer" :data-tip="t('jobs.answer')" :aria-label="t('jobs.answer')" @click="act(j, 'answer', $event)"><G n="IconHelp" /></button>
            <button v-else class="btn icon" data-jact="pause" :data-tip="j.state === 'paused' ? t('jobs.resume') : t('jobs.pause')" :aria-label="j.state === 'paused' ? t('jobs.resume') : t('jobs.pause')" @click="act(j, 'pause', $event)"><G :n="j.state === 'paused' ? 'IconPlay' : 'IconPause'" /></button>
            <button class="btn icon" data-jact="cancel" :data-tip="t('jobs.cancel')" :aria-label="t('jobs.cancel')" @click="act(j, 'cancel', $event)"><G n="IconStop" /></button>
          </template>
          <button v-else-if="j.state === 'failed' || j.state === 'cancelled'" class="btn icon" data-jact="retry" :data-tip="t('jobs.retry')" :aria-label="t('jobs.retry')" @click="act(j, 'retry', $event)"><G n="IconRetry" /></button>
        </span>
      </div>
    </template>
    <div v-else class="empty"><G n="IconQueued" /><div>{{ t('jobs.empty') }}</div></div>
  </div>
</template>
