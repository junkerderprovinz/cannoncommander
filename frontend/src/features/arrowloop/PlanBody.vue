<script setup lang="ts">
import { FieldRow, Selector, TextField } from '@glimstone/vue';
import { computed } from 'vue';
import type { ArrowLoopJob } from '@/api/tools';
import { t } from '@/i18n';
import { pressPrimary, segItems } from '@/ui/kit';

export interface PlanState { name: string; every: ArrowLoopJob['every']; at: string }
defineProps<{ st: PlanState }>();
const every = computed(() => (['hourly', 'daily', 'weekly'] as const).map((e) => [e, t(`arrowloop.${e}`)] as const));
</script>

<template>
  <div class="well glim-well" @keydown.enter="pressPrimary">
    <FieldRow :label="t('arrowloop.name')" :hint="t('arrowloop.nameTip')"><TextField id="al-name" v-model="st.name" width="medium" /></FieldRow>
    <FieldRow :label="t('arrowloop.every')"><Selector id="al-when" v-model="st.every" :items="segItems(every)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="t('arrowloop.at')" :hint="t('arrowloop.atTip')"><TextField id="al-time" v-model="st.at" maxlength="5" width="short" mono /></FieldRow>
  </div>
</template>
