<script setup lang="ts">
import { Button, Card, CopyField } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { admin, type AppChange, type AppSettings, type System } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { shell } from '@/shell/state';
import Frow from '@/ui/Frow.vue';
import { c, confirmWin, since } from '@/features/config/shared';
import Seg from '@/ui/Seg.vue';
import Sw from '@/ui/Sw.vue';
import { copyWords } from '@/ui/kit';
import ExportWin from '../wins/ExportWin.vue';
import ImportWin from '../wins/ImportWin.vue';
import { IMAGE } from '../support';

const app = ref<AppSettings | null>(null), sys = ref<System | null>(null);
const update = ref<'idle' | 'busy' | string>('idle');
const restarting = ref(false);
const load = () => Promise.all([admin.app().then((a) => { app.value = a; }), admin.system().then((s) => { sys.value = s; })]);
onMounted(() => { load().catch(() => {}); });

const save = (change: AppChange) => admin.saveApp(change).then((a) => { app.value!.restartNeeded = a.restartNeeded; toast(c('saved')); }, () => toast(c('notSaved'), 'warn'));
const autoCheck = computed({ get: () => !!app.value?.updates.autoCheck, set: (v: boolean) => { app.value!.updates.autoCheck = v; save({ updates: { autoCheck: v } }); } });
const logLevel = computed({ get: () => app.value?.logLevel ?? 'info', set: (v: AppSettings['logLevel']) => { app.value!.logLevel = v; save({ logLevel: v }); } });

async function check() {
  update.value = 'busy';
  const got = await admin.checkUpdate().catch(() => null);
  update.value = got ? (got.updateAvailable ? c('app.newer', { v: got.latest }) : c('app.current', { v: got.latest })) : 'idle';
  if (!got) toast(c('app.noAnswer'), 'warn');
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
      <Frow :label="c('app.image')"><CopyField class="w" :value="IMAGE" :label="c('app.image')" :text="copyWords()" /></Frow>
      <Frow label="Tag" :tip="c('app.tagTip')"><span class="pill mono">{{ sys?.version ?? '' }}</span></Frow>
      <div class="a-foot">
        <button id="a-upd" class="btn" :class="{ ok: update !== 'idle' && update !== 'busy' }" :disabled="update === 'busy'" @click="check"><G :n="update !== 'idle' && update !== 'busy' ? 'IconUpToDate' : 'IconSync'" />{{ update === 'idle' ? c('app.check') : update === 'busy' ? c('app.checking') : update }}</button>
      </div>
    </Card>
    <Card :title="c('app.backup')">
      <Frow :label="c('app.toFile')" :tip="c('app.toFileTip')"><Button id="a-export" :label="c('app.export')" label-key="IconExport" @click="win({ title: c('app.exportTitle'), body: ExportWin })" /></Frow>
      <Frow :label="c('app.fromFile')" :tip="c('app.fromFileTip')"><Button id="a-import" :label="c('app.import')" label-key="IconImport" @click="win({ title: c('app.importTitle'), body: ImportWin })" /></Frow>
    </Card>
    <Card :title="c('app.service')">
      <Frow :label="c('app.log')" :tip="c('app.logTip')"><Seg v-model="logLevel" :options="[['error', c('app.errors')], ['info', c('app.normal')], ['debug', c('app.verbose')]]" /></Frow>
      <Frow :label="c('app.running')" :tip="app.restartNeeded ? c('app.restartNeededTip') : undefined"><span class="a-ctl"><span class="pill" :class="{ warn: app.restartNeeded }">{{ app.restartNeeded ? c('app.restartNeeded') : sys ? since(sys.started) : '' }}</span><Button :label="c('app.restart')" label-key="IconRefresh" @click="restart" /></span></Frow>
    </Card>
  </template>
  <Teleport v-if="restarting" to="body">
    <div class="scrim glim-modal-backdrop a-veil"><div class="glim-card a-veil-card"><G n="IconRefresh" /><span>{{ c('app.restarting') }}</span></div></div>
  </Teleport>
</template>
