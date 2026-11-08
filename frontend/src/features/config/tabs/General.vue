<script setup lang="ts">
import { pathField } from '@/ui/picker';
import { segItems } from '@/ui/kit';
import { Button, Card, FieldRow, FieldRowGroup, PathField, Selector, Toggle } from '@glimstone/vue';
import { defineAsyncComponent, ref } from 'vue';
import { admin, exists } from '@/api/admin';
import { host, windowsPaths } from '@/core/host';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { useUnraid } from '@/features/unraid/state';
import { nativePath, typedPath } from '@/files/paths';
import { useUser } from '@/stores/user';
import { c, cn, fail, P } from '@/features/config/shared';

// The container build leaves the desktop app's rows out.
const DesktopRows = import.meta.env.DESKTOP ? defineAsyncComponent(() => import('@/features/desktop/DesktopRows.vue')) : null;
const U = useUnraid();
const home = useUser().me.home || '/';
// The desktop app shows and takes the folder the way the computer writes it.
const start = ref(nativePath(prefs.startFolder || home, windowsPaths()));
const field = ref<InstanceType<typeof PathField> | null>(null);
const CAPS: [number, string][] = [[0, c('general.noCap')], [25, '25 MB/s'], [50, '50 MB/s'], [100, '100 MB/s'], [200, '200 MB/s']];
// Only the desktop app can hand a file to the computer's own programs, and there it needs no download.
const DBLCLICK = (host.desktop ? [['system', c('general.system')], ['open', c('general.open')], ['preview', c('general.preview')]]
  : [['open', c('general.open')], ['preview', c('general.preview')], ['download', c('general.download')]]) as [string, string][];
const AFTER: [number, string][] = [[0, c('general.always')], [60, c('general.after1m')], [600, c('general.after10m')], [3600, c('general.after1h')]];

async function save() {
  const v = host.desktop ? typedPath(start.value, { windows: windowsPaths(), home }) : start.value.trim();
  if (!(await exists(v))) return fail(c('noFolder'), field.value?.input);
  setPref('startFolder', v);
  start.value = nativePath(v, windowsPaths());
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
    <FieldRowGroup>
      <FieldRow :label="c('general.start')" :hint="c('general.startTip')"><PathField ref="field" v-model="start" v-bind="pathField('a-start')" @change="save" @picked="save" /></FieldRow>
      <FieldRow :label="c('general.dblclick')" :hint="c('general.dblclickTip')"><Selector v-model="P.dblclick" :items="segItems(DBLCLICK)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
      <DesktopRows v-if="DesktopRows && host.desktop" />
    </FieldRowGroup>
  </Card>
  <Card :title="c('general.delete')">
    <FieldRow :label="c('general.confirm')" :hint="c('general.confirmTip')"><Toggle v-model="P.confirmDelete" /></FieldRow>
    <FieldRow :label="c('general.trash')" :hint="c('general.trashTip')"><Toggle v-model="P.trash" /></FieldRow>
    <FieldRow v-if="P.trash && !host.desktop" :label="c('general.keep')" :hint="c('general.keepTip')"><Selector v-model="P.trashDays" :items="segItems([7, 14, 30, 90].map((n) => [n, cn(n, 'general.days')]))" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
  </Card>
  <Card :title="c('general.jobs')">
    <FieldRow :label="c('general.jobMode')" :hint="c('general.jobModeTip')"><Selector v-model="P.jobMode" :items="segItems([['serial', c('general.serial')], ['parallel', c('general.parallel')]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="c('general.speedCap')" :hint="c('general.speedCapTip')"><Selector v-model="P.speedCap" :items="segItems(CAPS)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow v-if="!U.st || U.available" :label="c('general.parityWait')" :hint="c('general.parityWaitTip')"><Toggle :model-value="P.parityWait !== false" @update:model-value="P.parityWait = $event" /></FieldRow>
    <FieldRow :label="c('general.notifyDone')" :hint="c('general.notifyDoneTip')"><Toggle :model-value="!!P.notifyDone" @update:model-value="notifyDone" /></FieldRow>
    <!-- Off Unraid, as in the desktop app, there is no Unraid to tell. -->
    <FieldRow v-if="!U.st || U.available" :label="c('general.notifyUnraid')" :hint="c('general.notifyUnraidTip')"><Toggle v-model="P.notifyUnraid" /></FieldRow>
    <template v-if="P.notifyDone || P.notifyUnraid">
      <FieldRow :label="c('general.notifyAfter')" :hint="c('general.notifyAfterTip')"><Selector v-model="P.notifyAfter" :items="segItems(AFTER)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
      <FieldRow :label="c('general.tryIt')"><Button id="f-notify-test" :label="c('general.sendTest')" label-key="IconSendTest" @click="testNote" /></FieldRow>
    </template>
  </Card>
</template>
