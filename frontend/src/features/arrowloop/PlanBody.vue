<script setup lang="ts">
import { computed } from 'vue';
import type { ArrowLoopJob } from '@/api/tools';
import { t } from '@/i18n';
import Frow from '@/ui/Frow.vue';
import { pressPrimary } from '@/ui/kit';
import Seg from '@/ui/Seg.vue';

export interface PlanState { name: string; every: ArrowLoopJob['every']; at: string }
defineProps<{ st: PlanState }>();
const every = computed(() => (['hourly', 'daily', 'weekly'] as const).map((e) => [e, t(`arrowloop.${e}`)] as const));
</script>

<template>
  <div class="well" @keydown.enter="pressPrimary">
    <Frow :label="t('arrowloop.name')" :tip="t('arrowloop.nameTip')"><input id="al-name" v-model="st.name" class="field m"></Frow>
    <Frow :label="t('arrowloop.every')"><Seg id="al-when" v-model="st.every" :options="every" /></Frow>
    <Frow :label="t('arrowloop.at')" :tip="t('arrowloop.atTip')"><input id="al-time" v-model="st.at" class="field s mono" maxlength="5"></Frow>
  </div>
</template>
