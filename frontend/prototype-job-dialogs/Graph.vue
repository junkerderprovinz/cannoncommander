<script setup lang="ts">
// Prototype: throughput over the course of a job. Progress runs along x, speed up the one axis in MB/s.
import { computed, ref, useId } from 'vue';
import { fmtNumber, fmtSeconds, type Lang, type Sample, samplesTo, type State, type Texts } from './data';

const props = defineProps<{ base: number; totalMB: number; pct: number; state: State; lang: Lang; t: Texts; current?: string }>();

const samples = computed(() => samplesTo(props.base, props.totalMB, props.pct));
const last = computed(() => samples.value.at(-1)!);
const top = computed(() => {
  const peak = Math.max(...samples.value.map((s) => s.mbs), props.base);
  const step = peak > 120 ? 50 : 25;
  return Math.ceil((peak * 1.12) / step) * step;
});
const y = (mbs: number) => 100 - (mbs / top.value) * 100;
const line = computed(() => samples.value.map((s, i) => `${i ? 'L' : 'M'}${s.x.toFixed(2)},${y(s.mbs).toFixed(2)}`).join(''));
const area = computed(() => `${line.value}L${last.value.x.toFixed(2)},100L0,100Z`);
const live = computed(() => props.state === 'running');
const ended = computed(() => ['done', 'errors', 'cancelled'].includes(props.state));
const leftS = computed(() => (props.totalMB * (100 - props.pct)) / 100 / last.value.mbs);
const doneMB = computed(() => (props.totalMB * props.pct) / 100);
const gb = (mb: number) => `${fmtNumber(mb / 1024, 1, props.lang)} GB`;

const fill = `gr-fill-${useId()}`;
const hover = ref<Sample | null>(null);
function point(e: PointerEvent) {
  const box = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const x = Math.min(props.pct, Math.max(0, ((e.clientX - box.left) / box.width) * 100));
  hover.value = samples.value.reduce((a, b) => (Math.abs(b.x - x) < Math.abs(a.x - x) ? b : a));
}
const tipSide = computed(() => (hover.value && hover.value.x > 70 ? 'end' : 'start'));
</script>

<template>
  <div class="gr" :class="{ live, ended }">
    <div class="gr-chart">
      <div class="gr-plot" tabindex="0" role="img"
        :aria-label="`${t.speed}: ${fmtNumber(last.mbs, 0, lang)} MB/s, ${t.progress} ${Math.round(pct)} %`"
        @pointermove="point" @pointerleave="hover = null" @blur="hover = null">
        <svg viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
          <defs>
            <linearGradient :id="fill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" class="gr-stop-top" />
              <stop offset="1" class="gr-stop-bottom" />
            </linearGradient>
          </defs>
          <line class="gr-grid" x1="0" x2="100" y1="0" y2="0" />
          <line class="gr-grid" x1="0" x2="100" y1="50" y2="50" />
          <line class="gr-base" x1="0" x2="100" y1="100" y2="100" />
          <path class="gr-area" :d="area" :fill="`url(#${fill})`" />
          <path class="gr-line" :d="line" />
        </svg>
        <span class="gr-point" :style="{ left: `${last.x}%`, top: `${y(last.mbs)}%` }"><i></i></span>
        <template v-if="hover">
          <span class="gr-cross" :style="{ left: `${hover.x}%` }"></span>
          <span class="gr-mark" :style="{ left: `${hover.x}%`, top: `${y(hover.mbs)}%` }"></span>
          <span class="gr-tip" :class="`gr-tip--${tipSide}`" :style="{ left: `${hover.x}%` }">
            <b class="glim-num">{{ fmtNumber(hover.mbs, 0, lang) }} MB/s</b>
            <span class="glim-num">{{ t.after }} {{ fmtSeconds(hover.s, lang) }} · {{ fmtNumber(hover.x, 0, lang) }} %</span>
          </span>
        </template>
        <span class="gr-tick glim-num" style="top: 0" aria-hidden="true">{{ top }} MB/s</span>
        <span class="gr-tick glim-num" style="top: 50%" aria-hidden="true">{{ top / 2 }}</span>
      </div>
      <div class="gr-x glim-num" aria-hidden="true"><span>0 %</span><span>100 %</span></div>
    </div>
    <dl class="gr-read">
      <div><dt>{{ t.speed }}</dt><dd class="glim-num">{{ live ? `${fmtNumber(last.mbs, 0, lang)} MB/s` : ended ? '…' : t.badge.paused }}</dd></div>
      <div><dt>{{ t.left }}</dt><dd class="glim-num">{{ ended ? '…' : fmtSeconds(leftS, lang) }}</dd></div>
      <div><dt>{{ t.size }}</dt><dd class="glim-num">{{ gb(doneMB) }} <span>{{ t.of }} {{ gb(totalMB) }}</span></dd></div>
    </dl>
    <p v-if="current && !ended" class="gr-now"><span>{{ t.now }}</span><span class="mono">{{ current }}</span></p>
  </div>
</template>
