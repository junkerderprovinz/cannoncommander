<script setup lang="ts">
// Prototype: the bar of a running job, which turns into the throughput graph when the window is opened up.
import { ProgressBar } from '@glimstone/vue';
import { computed } from 'vue';
import { G } from '@/core/glyphs';
import Graph from './Graph.vue';
import { currentFile, type Dialog, JOBS, type Lang, setDetails, type State, type Texts, ui } from './data';

const props = defineProps<{ dialog: Dialog; state: State; pct: number; lang: Lang; t: Texts; label: string; stacked?: boolean; marker?: boolean; inline?: boolean }>();
const job = computed(() => JOBS[props.dialog]);
const open = computed(() => ui.details);
const items = computed(() => ('items' in job.value ? job.value.items : []) as readonly string[]);
const doneCount = computed(() => Math.round((items.value.length * props.pct) / 100));
</script>

<template>
  <div class="meter" :class="{ open }">
    <div class="xp xp--bar" :class="{ shut: open }" :aria-hidden="open">
      <div class="xp-bar"><slot name="lead" /><ProgressBar :value="pct / 100" :label="label" :paused="state === 'paused'" :failed="state === 'errors'" /><slot name="facts" /></div>
    </div>
    <div class="xp xp--more" :class="{ shut: !open }" :aria-hidden="!open">
      <div>
        <Graph v-if="dialog !== 'delete'" :base="job.speed" :total-m-b="job.totalMB" :pct="pct" :state="state" :lang="lang" :t="t" :current="currentFile(job.files)" :stacked="stacked" :marker="marker" :inline="inline" />
        <ul v-else class="mt-items">
          <li v-for="(n, i) in items" :key="n" :class="{ on: i < doneCount, now: i === doneCount && state === 'running' }">
            <span class="mt-mark"><G :n="i < doneCount ? 'IconCheck' : 'IconFile'" /></span>
            <span class="mt-name">{{ n }}</span>
            <span class="mt-state">{{ i < doneCount ? t.trash : i === doneCount && state === 'running' ? t.badge.running : '' }}</span>
          </li>
        </ul>
        <slot />
      </div>
    </div>
    <button v-if="!inline" class="meter-toggle" :aria-expanded="open" @click="setDetails(!open)">
      <G n="IconExpand" />{{ open ? t.lessDetails : t.moreDetails }}
    </button>
  </div>
</template>
