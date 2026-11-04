<script setup lang="ts">
import { computed } from 'vue';
import { t } from '@/i18n';
import Frow from '@/ui/Frow.vue';
import Seg from '@/ui/Seg.vue';

defineProps<{ st: { name: string; every: 'hourly' | 'daily' | 'weekly'; at: string } }>();
const EVERY = computed(() => [['hourly', t('compare.hourly')], ['daily', t('compare.daily')], ['weekly', t('compare.weekly')]] as const);
</script>

<template>
  <div class="well">
    <Frow :label="t('compare.alName')" :tip="t('compare.alNameTip')"><input id="al-name" v-model="st.name" class="field m"></Frow>
    <Frow :label="t('compare.alWhen')"><Seg id="al-when" v-model="st.every" :options="EVERY" /></Frow>
    <Frow :label="t('compare.alTime')" :tip="t('compare.alTimeTip')"><input id="al-time" v-model="st.at" class="field s mono" maxlength="5"></Frow>
  </div>
</template>
