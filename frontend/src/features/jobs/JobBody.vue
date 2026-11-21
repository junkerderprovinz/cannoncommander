<script setup lang="ts">
// The job a window started. Small, it shows the way the files take and a bar; opened up, the speed over the course of
// the job with its numbers on one line, and whether the window stays once the job is through.
import { Checkbox, ProgressBar } from '@glimstone/vue';
import { computed } from 'vue';
import { fmtLeft, fmtSize } from '@/core/format';
import { prefs, setPref } from '@/core/prefs';
import { split } from '@/files/paths';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import Flight from './Flight.vue';
import { factsOf, jobLabel, pctOf, placeName } from './label';
import Throughput from './Throughput.vue';

const props = defineProps<{ id: string; from?: string }>();
const jobs = useJobs();
const j = computed(() => jobs.find(props.id));
const open = computed(() => prefs.jobDetails === true);
const keep = computed({ get: () => prefs.keepJobWindow === true, set: (on: boolean) => setPref('keepJobWindow', on) });
const ended = computed(() => !!j.value && ['done', 'failed', 'cancelled'].includes(j.value.state));
const trash = computed(() => j.value?.kind === 'trash');
const gone = computed(() => j.value?.kind === 'delete');
const bytes = computed(() => !!j.value?.totalBytes && !trash.value && !gone.value);
const to = computed(() => (trash.value ? t('jobs.trash') : placeName(j.value?.dest)));
const file = computed(() => (j.value?.current ? split(j.value.current).pop() : ''));
const line = computed(() => {
  const x = j.value!;
  if (!bytes.value) return [t('jobs.itemsDone', { done: x.doneFiles, total: x.totalFiles }), x.state === 'running' ? '' : t(`jobs.state.${x.state}`)];
  return [
    x.state === 'running' && x.rate ? t('jobs.perSecond', { size: fmtSize(x.rate) }) : t(`jobs.state.${x.state}`),
    x.state === 'running' && x.left != null ? fmtLeft(x.left) : '',
    t('jobs.of', { done: fmtSize(x.doneBytes) || '0 B', total: fmtSize(x.totalBytes) || '0 B' }),
  ];
});
</script>

<template>
  <div v-if="j" class="jf" :class="{ open }" role="status">
    <div class="jf-head"><b>{{ jobLabel(j) }}</b><span class="jf-pct glim-num">{{ Math.round(pctOf(j)) }} %</span></div>
    <div v-if="ended" class="jf-result glim-num">{{ factsOf(j) }}</div>
    <div class="jf-xp" :class="{ shut: open }" :aria-hidden="open">
      <div class="jf-small">
        <Flight v-if="!ended && from && !gone" :from="placeName(from)" :to="to" :to-glyph="trash ? 'IconTrash' : undefined" :file="j.current" :moving="j.state === 'running'" :paused="j.state === 'paused'" />
        <ProgressBar :value="pctOf(j) / 100" :label="jobLabel(j)" :paused="j.state === 'paused'" :failed="j.state === 'failed'" />
      </div>
    </div>
    <div class="jf-xp" :class="{ shut: !open }" :aria-hidden="!open">
      <div class="jf-large">
        <Throughput v-if="bytes" :job="j" />
        <p class="jf-line glim-num">
          <span v-for="(part, k) in line.filter(Boolean)" :key="k">{{ part }}</span>
          <span v-if="file && !ended" class="jf-file mono">{{ file }}</span>
        </p>
        <Checkbox v-model="keep" class="jf-keep" :label="t('jobs.keepOpen')" />
      </div>
    </div>
  </div>
</template>
