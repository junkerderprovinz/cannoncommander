<script setup lang="ts">
// The speed of a job over its course: progress runs along x, speed up the one axis, in the accent with a soft fill.
import { computed, ref, useId } from 'vue';
import type { Job } from '@/api/ops';
import { fmtSize, MB } from '@/core/format';
import { t } from '@/i18n';
import { pctOf, took } from './label';
import { type Sample, samplesOf } from './throughput';

const props = defineProps<{ job: Job }>();

const live = computed(() => props.job.state === 'running');
const points = computed<Sample[]>(() => {
  const list = samplesOf(props.job.id);
  const pct = pctOf(props.job), last = list.at(-1);
  return live.value && props.job.rate && (!last || pct > last.pct) ? [...list, { pct, rate: props.job.rate, at: Date.now() }] : list;
});
const top = computed(() => {
  const peak = Math.max(0, ...points.value.map((s) => s.rate)) / MB;
  const step = peak > 120 ? 50 : peak > 40 ? 25 : 5;
  return Math.max(step, Math.ceil((peak * 1.12) / step) * step);
});
const y = (rate: number) => 100 - (rate / MB / top.value) * 100;
const line = computed(() => points.value.map((s, i) => `${i ? 'L' : 'M'}${s.pct.toFixed(2)},${y(s.rate).toFixed(2)}`).join(''));
const area = computed(() => (points.value.length ? `${line.value}L${points.value.at(-1)!.pct.toFixed(2)},100L${points.value[0]!.pct.toFixed(2)},100Z` : ''));
const end = computed(() => points.value.at(-1));
const fill = `jgr-fill-${useId()}`;

const hover = ref<Sample | null>(null);
function point(e: PointerEvent) {
  if (!points.value.length) return;
  const box = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const x = ((e.clientX - box.left) / box.width) * 100;
  hover.value = points.value.reduce((a, b) => (Math.abs(b.pct - x) < Math.abs(a.pct - x) ? b : a));
}
const since = (s: Sample) => took(Math.max(0, (s.at - points.value[0]!.at) / 1000));
</script>

<template>
  <div class="jgr" :class="{ live }">
    <div class="jgr-plot" tabindex="0" role="img" :aria-label="t('jobs.graph', { speed: fmtSize(job.rate) || '0 B', pct: Math.round(pctOf(job)) })"
      @pointermove="point" @pointerleave="hover = null" @blur="hover = null">
      <svg viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient :id="fill" x1="0" y1="0" x2="0" y2="1"><stop offset="0" class="jgr-stop-top" /><stop offset="1" class="jgr-stop-bottom" /></linearGradient>
        </defs>
        <line class="jgr-grid" x1="0" x2="100" y1="0" y2="0" />
        <line class="jgr-grid" x1="0" x2="100" y1="50" y2="50" />
        <line class="jgr-base" x1="0" x2="100" y1="100" y2="100" />
        <path v-if="area" class="jgr-area" :d="area" :fill="`url(#${fill})`" />
        <path v-if="line" class="jgr-line" :d="line" />
      </svg>
      <span class="jgr-tick glim-num" style="top: 0" aria-hidden="true">{{ top }} MB/s</span>
      <span class="jgr-tick glim-num" style="top: 50%" aria-hidden="true">{{ top / 2 }}</span>
      <span v-if="end" class="jgr-point" :style="{ left: `${end.pct}%`, top: `${y(end.rate)}%` }"><i></i></span>
      <template v-if="hover">
        <span class="jgr-cross" :style="{ left: `${hover.pct}%` }"></span>
        <span class="jgr-mark" :style="{ left: `${hover.pct}%`, top: `${y(hover.rate)}%` }"></span>
        <span class="jgr-tip" :class="{ 'jgr-tip--end': hover.pct > 70 }" :style="{ left: `${hover.pct}%` }">
          <b class="glim-num">{{ t('jobs.perSecond', { size: fmtSize(hover.rate) }) }}</b>
          <span class="glim-num">{{ t('jobs.after', { time: since(hover), pct: Math.round(hover.pct) }) }}</span>
        </span>
      </template>
    </div>
    <div class="jgr-x glim-num" aria-hidden="true"><span>0 %</span><span>100 %</span></div>
  </div>
</template>
