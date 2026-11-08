<script setup lang="ts">
import { Button, Card, FieldRow, Pill, Selector } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { admin, type AppChange, type AppSettings, type System } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { shell } from '@/shell/state';
import { c, cn, confirmWin, since } from '@/features/config/shared';
import { segItems } from '@/ui/kit';
import ExportWin from '../wins/ExportWin.vue';
import ImportWin from '../wins/ImportWin.vue';

const app = ref<AppSettings | null>(null), sys = ref<System | null>(null);
const restarting = ref(false);
const load = () => Promise.all([admin.app().then((a) => { app.value = a; }), admin.system().then((s) => { sys.value = s; })]);
onMounted(() => { load().catch(() => {}); });

const save = (change: AppChange) => admin.saveApp(change).then(() => toast(c('saved')), () => toast(c('notSaved'), 'warn'));
const logLevel = computed({ get: () => app.value?.logLevel ?? 'info', set: (v: AppSettings['logLevel']) => { app.value!.logLevel = v; save({ logLevel: v }); } });

function restart() {
  const n = shell.jobCount;
  confirmWin({
    title: c('app.restart'), verb: c('app.restart'), glyph: 'IconRefresh',
    text: [c('app.restartText'), ...(n ? [cn(n, 'app.restartJobs')] : [])],
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
    <Card :title="c('app.backup')">
      <FieldRow :label="c('app.toFile')" :hint="c('app.toFileTip')"><Button id="a-export" :label="c('app.export')" label-key="IconExport" @click="win({ title: c('app.exportTitle'), body: ExportWin })" /></FieldRow>
      <FieldRow :label="c('app.fromFile')" :hint="c('app.fromFileTip')"><Button id="a-import" :label="c('app.import')" label-key="IconImport" @click="win({ title: c('app.importTitle'), body: ImportWin })" /></FieldRow>
    </Card>
    <Card :title="c('app.service')">
      <FieldRow :label="c('app.log')" :hint="c('app.logTip')"><Selector v-model="logLevel" :items="segItems([['error', c('app.errors')], ['info', c('app.normal')], ['debug', c('app.verbose')]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
      <FieldRow :label="c('app.running')"><Pill>{{ sys ? since(sys.started) : '' }}</Pill><Button :label="c('app.restart')" label-key="IconRefresh" @click="restart" /></FieldRow>
    </Card>
  </template>
  <Teleport v-if="restarting" to="body">
    <div class="scrim glim-modal-backdrop a-veil"><div class="glim-card a-veil-card"><G n="IconRefresh" /><span>{{ c('app.restarting') }}</span></div></div>
  </Teleport>
</template>
