<script setup lang="ts">
// The server writes the file into a folder of its own, not into the browser's downloads, so it can be found and
// imported again without leaving CannonCommander.
import { onMounted, ref } from 'vue';
import { admin, exists, listDir } from '@/api/admin';
import { go } from '@/core/views';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { usePanels } from '@/stores/panels';
import CopyField from '../kit/CopyField.vue';
import Frow from '../kit/Frow.vue';
import { c, fail, setFoot } from '../kit/kit';
import PathField from '../kit/PathField.vue';
import Sw from '../kit/Sw.vue';

const props = defineProps<{ win: WinEntry }>();
const today = new Date().toISOString().slice(0, 10);
const dir = ref('/mnt/user/appdata/cannoncommander/exports'), name = ref(c('app.exportName', { date: today })), users = ref(true);
const written = ref<string | null>(null);
const at = (sel: string) => props.win.el?.querySelector(sel) ?? null;

async function save() {
  const d = dir.value.trim().replace(/\/+$/, ''), n = name.value.trim();
  if (!(await exists(d))) return fail(c('noFolder'), at('#a-exdir'));
  if (!/^[^/\\]+\.json$/.test(n)) return fail(c('app.endsJson'), at('#a-exname'));
  if ((await listDir(d)).some((k) => k.name === n)) return fail(c('app.fileExists'), at('#a-exname'));
  const full = `${d}/${n}`;
  if (!(await admin.exportSettings(full, users.value).then(() => true, () => false))) return fail(c('notSaved'), at('#a-exname'));
  written.value = full;
  setFoot(props.win, [
    { label: c('app.showFile'), g: 'IconFolderOpen', run: () => { go('files'); usePanels().go(d); } },
    { label: c('done'), g: 'IconCheck', primary: true },
  ]);
}

onMounted(() => setFoot(props.win, [cancelBtn(), { label: c('app.export'), g: 'IconExport', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <div v-if="written" class="well"><Frow :label="c('app.savedAt')"><CopyField :value="written" grow /></Frow></div>
  <template v-else>
    <div class="well">
      <Frow :label="c('picker.folder')" :tip="c('app.exportDirTip')"><PathField id="a-exdir" v-model="dir" /></Frow>
      <Frow :label="c('app.fileName')"><input id="a-exname" v-model="name" class="field w"></Frow>
    </div>
    <div class="well">
      <Frow :label="c('app.withUsers')"><Sw v-model="users" /></Frow>
      <Frow :label="c('app.secrets')" :tip="c('app.secretsTip')"><span class="pill">{{ c('app.stayOut') }}</span></Frow>
    </div>
  </template>
</template>
