<script setup lang="ts">
import { Card } from '@glimstone/vue';
import { ref } from 'vue';
import { admin, exists } from '@/api/admin';
import { G } from '@/core/glyphs';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { useUser } from '@/stores/user';
import Frow from '@/ui/Frow.vue';
import { c, fail, P } from '@/features/config/shared';
import PathField from '@/ui/PathField.vue';
import Seg from '@/ui/Seg.vue';
import Sw from '@/ui/Sw.vue';

const start = ref(prefs.startFolder || '/mnt/user/downloads');
const field = ref<InstanceType<typeof PathField> | null>(null);
const CAPS: [number, string][] = [[0, c('general.noCap')], [25, '25 MB/s'], [50, '50 MB/s'], [100, '100 MB/s'], [200, '200 MB/s']];
const AFTER: [number, string][] = [[0, c('general.always')], [60, c('general.after1m')], [600, c('general.after10m')], [3600, c('general.after1h')]];

async function save() {
  const v = start.value.trim();
  if (!(await exists(v))) return fail(c('noFolder'), field.value?.input);
  setPref('startFolder', v);
  toast(c('general.startSaved'));
}

async function allowNotes() {
  if (typeof Notification === 'undefined') { toast(c('general.noNotes'), 'warn'); return false; }
  if (Notification.permission === 'default') await Notification.requestPermission();
  if (Notification.permission === 'granted') return true;
  toast(c('general.notesBlocked'), 'warn');
  return false;
}
// The switch goes back off when the browser does not allow notifications, so it never promises one that cannot come.
async function notifyDone(on: boolean) {
  P.notifyDone = on;
  if (on && !(await allowNotes())) P.notifyDone = false;
}
async function testNote() {
  if (P.notifyDone) {
    if (!(await allowNotes())) return;
    new Notification('CannonCommander', { body: c('general.testBody'), tag: 'cannoncommander-job' });
  }
  if (P.notifyUnraid && useUser().admin()) {
    const sent = await admin.notifyUnraid('CannonCommander', c('general.testBody')).then(() => true, () => false);
    if (!sent) return toast(c('general.unraidFailed'), 'warn');
  }
  toast(P.notifyDone && P.notifyUnraid ? c('general.testBoth') : P.notifyUnraid ? c('general.testUnraid') : c('general.testSent'));
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
    <Frow :label="c('general.speedCap')" :tip="c('general.speedCapTip')"><Seg v-model="P.speedCap" :options="CAPS" /></Frow>
    <Frow :label="c('general.parityWait')" :tip="c('general.parityWaitTip')"><Sw :model-value="P.parityWait !== false" @update:model-value="P.parityWait = $event" /></Frow>
    <Frow :label="c('general.notifyDone')" :tip="c('general.notifyDoneTip')"><Sw :model-value="!!P.notifyDone" @update:model-value="notifyDone" /></Frow>
    <Frow :label="c('general.notifyUnraid')" :tip="c('general.notifyUnraidTip')"><Sw v-model="P.notifyUnraid" /></Frow>
    <template v-if="P.notifyDone || P.notifyUnraid">
      <Frow :label="c('general.notifyAfter')" :tip="c('general.notifyAfterTip')"><Seg v-model="P.notifyAfter" :options="AFTER" /></Frow>
      <Frow :label="c('general.tryIt')"><button id="f-notify-test" class="btn" @click="testNote"><G n="IconSendTest" />{{ c('general.sendTest') }}</button></Frow>
    </template>
  </Card>
</template>
