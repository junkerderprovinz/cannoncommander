<script setup lang="ts">
// The job a window started, as the jobs window shows it, and whether the window stays once the job is through.
import { Checkbox, Pill, ProgressBar } from '@glimstone/vue';
import { computed } from 'vue';
import { prefs, setPref } from '@/core/prefs';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { factsOf, jobLabel, pctOf } from './label';

const props = defineProps<{ id: string }>();
const jobs = useJobs();
const j = computed(() => jobs.find(props.id));
const keep = computed({ get: () => prefs.keepJobWindow === true, set: (on: boolean) => setPref('keepJobWindow', on) });
</script>

<template>
  <div v-if="j" class="jf" role="status">
    <div class="jf-head"><b>{{ jobLabel(j) }}</b><Pill class="glim-num">{{ Math.round(pctOf(j)) }} %</Pill></div>
    <ProgressBar :value="pctOf(j) / 100" :label="jobLabel(j)" :paused="j.state === 'paused'" :failed="j.state === 'failed'" />
    <div class="jf-facts glim-num">{{ factsOf(j) }}</div>
    <div v-if="j.current && j.state === 'running'" class="jf-current mono">{{ j.current }}</div>
  </div>
  <Checkbox v-model="keep" class="jf-keep" :label="t('jobs.keepOpen')" />
</template>
