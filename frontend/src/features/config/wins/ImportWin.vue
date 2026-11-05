<script setup lang="ts">
import { pathField } from '@/ui/picker';
import { FieldRow, PathField, Toggle } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { admin, exists, listDir } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, fail, stepFoot } from '@/features/config/shared';

const props = defineProps<{ win: WinEntry }>();
const EXPORTS = '/mnt/user/appdata/cannoncommander/exports';
const file = ref(''), users = ref(true);
const at = (sel: string) => props.win.el?.querySelector(sel) ?? null;

async function run() {
  const p = file.value.trim();
  if (!(await exists(p, 'file'))) return fail(c('app.noFile'), at('#a-imfile'));
  if (!p.toLowerCase().endsWith('.json')) return fail(c('app.notExport'), at('#a-imfile'));
  const got = await admin.importSettings(p, users.value).catch(() => null);
  if (!got) return fail(c('app.notExport'), at('#a-imfile'));
  props.win.close();
  toast(c(got.restartNeeded ? 'app.importedRestart' : 'app.imported', { name: p.split('/').pop()! }));
}

onMounted(async () => {
  stepFoot(props.win, [cancelBtn(), { label: c('app.import'), g: 'IconImport', primary: true, run: () => { run(); return false; } }]);
  // The newest export is the likeliest one to bring back, so it stands in the field already.
  const newest = (await listDir(EXPORTS).catch(() => [])).filter((k) => !k.dir && k.name.endsWith('.json')).map((k) => k.name).sort().pop();
  if (newest && !file.value) file.value = `${EXPORTS}/${newest}`;
});
</script>

<template>
  <div class="well glim-well">
    <FieldRow :label="c('picker.file')" :hint="c('app.importFileTip')"><PathField v-model="file" :placeholder="`${EXPORTS}/…json`" v-bind="pathField('a-imfile', ['json'])" /></FieldRow>
    <FieldRow :label="c('app.takeUsers')" :hint="c('app.takeUsersTip')"><Toggle v-model="users" /></FieldRow>
  </div>
</template>
