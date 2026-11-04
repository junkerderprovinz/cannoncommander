<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { admin, type AppSettings, type System } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { shell } from '@/shell/state';
import Btn from '../kit/Btn.vue';
import Card from '../kit/Card.vue';
import CopyField from '../kit/CopyField.vue';
import Frow from '../kit/Frow.vue';
import { c, confirmWin, since } from '../kit/kit';
import Seg from '../kit/Seg.vue';
import Sw from '../kit/Sw.vue';
import ExportWin from '../wins/ExportWin.vue';
import ImportWin from '../wins/ImportWin.vue';

const app = ref<AppSettings | null>(null), sys = ref<System | null>(null);
const update = ref<'idle' | 'busy' | string>('idle');
const restarting = ref(false);
const load = () => Promise.all([admin.app().then((a) => { app.value = a; }), admin.system().then((s) => { sys.value = s; })]);
onMounted(() => { load().catch(() => {}); });

const save = () => admin.saveApp(app.value!).then(() => toast(c('saved')), () => toast(c('notSaved'), 'warn'));
const autoCheck = computed({ get: () => !!app.value?.autoCheck, set: (v: boolean) => { app.value!.autoCheck = v; save(); } });
const logLevel = computed({ get: () => app.value?.logLevel ?? 'info', set: (v: AppSettings['logLevel']) => { app.value!.logLevel = v; save(); } });

async function check() {
  update.value = 'busy';
  const got = await admin.checkUpdate().catch(() => null);
  update.value = got ? (got.current ? c('app.current', { v: got.latest }) : c('app.newer', { v: got.latest })) : 'idle';
}

function restart() {
  const n = shell.jobCount;
  confirmWin({
    title: c('app.restart'), verb: c('app.restart'), glyph: 'IconRefresh',
    text: [c('app.restartText'), ...(n ? [n === 1 ? c('app.restartJob') : c('app.restartJobs', { n })] : [])],
    run: async () => {
      restarting.value = true;
      await admin.restart().catch(() => {});
      // The server is gone for a moment, so it is asked again until it answers.
      const back = async () => {
        if (!(await admin.system().then(() => true, () => false))) {
          setTimeout(back, 700);
          return;
        }
        restarting.value = false;
        await load();
        toast(c('app.back'));
      };
      setTimeout(back, 1500);
    },
  });
}
</script>

<template>
  <template v-if="app">
    <Card :title="c('app.updates')">
      <Frow :label="c('app.autoCheck')" :tip="c('app.autoCheckTip')"><Sw v-model="autoCheck" /></Frow>
      <Frow :label="c('app.image')"><CopyField :value="app.image" /></Frow>
      <Frow label="Tag" :tip="c('app.tagTip')"><span class="pill mono">{{ app.tag }}</span></Frow>
      <div class="a-foot">
        <button id="a-upd" class="btn" :class="{ ok: update !== 'idle' && update !== 'busy' }" :disabled="update === 'busy'" @click="check"><G :n="update !== 'idle' && update !== 'busy' ? 'IconUpToDate' : 'IconSync'" />{{ update === 'idle' ? c('app.check') : update === 'busy' ? c('app.checking') : update }}</button>
      </div>
    </Card>
    <Card :title="c('app.backup')">
      <Frow :label="c('app.toFile')" :tip="c('app.toFileTip')"><Btn :label="c('app.export')" g="IconExport" @click="win({ title: c('app.exportTitle'), body: ExportWin })" /></Frow>
      <Frow :label="c('app.fromFile')" :tip="c('app.fromFileTip')"><Btn :label="c('app.import')" g="IconImport" @click="win({ title: c('app.importTitle'), body: ImportWin })" /></Frow>
    </Card>
    <Card :title="c('app.service')">
      <Frow :label="c('app.log')" :tip="c('app.logTip')"><Seg v-model="logLevel" :options="[['error', c('app.errors')], ['info', c('app.normal')], ['debug', c('app.verbose')]]" /></Frow>
      <Frow :label="c('app.running')"><span class="a-ctl"><span class="pill">{{ sys ? since(sys.started) : '' }}</span><Btn :label="c('app.restart')" g="IconRefresh" @click="restart" /></span></Frow>
    </Card>
  </template>
  <Teleport v-if="restarting" to="body">
    <div class="scrim glim-modal-backdrop a-veil"><div class="glim-card a-veil-card"><G n="IconRefresh" /><span>{{ c('app.restarting') }}</span></div></div>
  </Teleport>
</template>
