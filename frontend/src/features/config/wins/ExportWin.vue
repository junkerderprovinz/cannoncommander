<script setup lang="ts">
// The server writes the file into a folder of its own, not into the browser's downloads, so it can be found and
// imported again without leaving CannonCommander.
import { pathField } from '@/ui/picker';
import { CopyField, FieldRow, PathField, Pill, TextField, Toggle } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { admin, exists, listDir, statusOf } from '@/api/admin';
import { go } from '@/core/views';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { usePanels } from '@/stores/panels';
import { c, fail, stepFoot } from '@/features/config/shared';
import { copyWords } from '@/ui/kit';

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
  try {
    written.value = (await admin.exportSettings(full, users.value)).path;
  } catch (e) {
    return fail(statusOf(e) === 409 ? c('app.fileExists') : c('notSaved'), at('#a-exname'));
  }
  stepFoot(props.win, [
    { label: c('app.showFile'), g: 'IconFolderOpen', run: () => { go('files'); usePanels().go(d); } },
    { label: c('done'), g: 'IconCheck', primary: true },
  ]);
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: c('app.export'), g: 'IconExport', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <div v-if="written" class="well glim-well"><FieldRow :label="c('app.savedAt')"><CopyField class="glim-field--grow" :value="written" :label="c('app.savedAt')" :text="copyWords()" /></FieldRow></div>
  <template v-else>
    <div class="well glim-well">
      <FieldRow :label="c('picker.folder')" :hint="c('app.exportDirTip')"><PathField v-model="dir" v-bind="pathField('a-exdir')" /></FieldRow>
      <FieldRow :label="c('app.fileName')"><TextField id="a-exname" v-model="name" width="wide" /></FieldRow>
    </div>
    <div class="well glim-well">
      <FieldRow :label="c('app.withUsers')"><Toggle v-model="users" /></FieldRow>
      <FieldRow :label="c('app.secrets')" :hint="c('app.secretsTip')"><Pill>{{ c('app.stayOut') }}</Pill></FieldRow>
    </div>
  </template>
</template>
