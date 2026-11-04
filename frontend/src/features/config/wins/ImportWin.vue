<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { admin, exists, listDir } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import Frow from '../kit/Frow.vue';
import { c, fail, setFoot } from '../kit/kit';
import PathField from '../kit/PathField.vue';
import Sw from '../kit/Sw.vue';

const props = defineProps<{ win: WinEntry }>();
const EXPORTS = '/mnt/user/appdata/cannoncommander/exports';
const file = ref(''), users = ref(true);
const at = (sel: string) => props.win.el?.querySelector(sel) ?? null;

async function run() {
  const p = file.value.trim();
  if (!(await exists(p, 'file'))) return fail(c('app.noFile'), at('#a-imfile'));
  if (!p.toLowerCase().endsWith('.json')) return fail(c('app.notExport'), at('#a-imfile'));
  if (!(await admin.importSettings(p, users.value).then(() => true, () => false))) return fail(c('app.notExport'), at('#a-imfile'));
  props.win.close();
  toast(c('app.imported', { name: p.split('/').pop()! }));
}

onMounted(async () => {
  setFoot(props.win, [cancelBtn(), { label: c('app.import'), g: 'IconImport', primary: true, run: () => { run(); return false; } }]);
  // The newest export is the likeliest one to bring back, so it stands in the field already.
  const newest = (await listDir(EXPORTS).catch(() => [])).filter((k) => !k.dir && k.name.endsWith('.json')).map((k) => k.name).sort().pop();
  if (newest && !file.value) file.value = `${EXPORTS}/${newest}`;
});
</script>

<template>
  <div class="well">
    <Frow :label="c('picker.file')" :tip="c('app.importFileTip')"><PathField id="a-imfile" v-model="file" :exts="['json']" :placeholder="`${EXPORTS}/…json`" /></Frow>
    <Frow :label="c('app.takeUsers')" :tip="c('app.takeUsersTip')"><Sw v-model="users" /></Frow>
  </div>
</template>
