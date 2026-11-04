<script setup lang="ts">
import { ref } from 'vue';
import { exists } from '@/api/admin';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import Card from '../kit/Card.vue';
import Frow from '../kit/Frow.vue';
import { c, fail, P } from '../kit/kit';
import PathField from '../kit/PathField.vue';
import Seg from '../kit/Seg.vue';
import Sw from '../kit/Sw.vue';

const start = ref(prefs.startFolder || '/mnt/user/downloads');
const field = ref<InstanceType<typeof PathField> | null>(null);

async function save() {
  const v = start.value.trim();
  if (!(await exists(v))) return fail(c('noFolder'), field.value?.input);
  setPref('startFolder', v);
  toast(c('general.startSaved'));
}
</script>

<template>
  <Card :title="c('general.use')">
    <Frow :label="c('general.start')" :tip="c('general.startTip')"><PathField id="a-start" ref="field" v-model="start" @change="save" @picked="save" /></Frow>
    <Frow :label="c('general.dblclick')" :tip="c('general.dblclickTip')"><Seg v-model="P.dblclick" :options="[['open', c('general.open')], ['preview', c('general.preview')], ['download', c('general.download')]]" /></Frow>
  </Card>
  <Card :title="c('general.delete')">
    <Frow :label="c('general.confirm')" :tip="c('general.confirmTip')"><Sw v-model="P.confirmDelete" /></Frow>
    <Frow :label="c('general.trash')" :tip="c('general.trashTip')"><Sw v-model="P.trash" /></Frow>
    <Frow v-if="P.trash" :label="c('general.keep')" :tip="c('general.keepTip')"><Seg v-model="P.trashDays" :options="[7, 14, 30, 90].map((n) => [n, c('general.days', { n })])" /></Frow>
  </Card>
  <Card :title="c('general.jobs')">
    <Frow :label="c('general.jobMode')" :tip="c('general.jobModeTip')"><Seg v-model="P.jobMode" :options="[['serial', c('general.serial')], ['parallel', c('general.parallel')]]" /></Frow>
  </Card>
</template>
