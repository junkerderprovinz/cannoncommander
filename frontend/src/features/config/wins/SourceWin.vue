<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { admin, exists, type Source } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import Frow from '../kit/Frow.vue';
import Info from '../kit/Info.vue';
import { c, fail, setFoot } from '../kit/kit';
import PathField from '../kit/PathField.vue';
import Sw from '../kit/Sw.vue';

const props = defineProps<{ win: WinEntry; source: Source | null; all: Source[]; saved: (reindexPath: string | null) => void }>();
const s = props.source;
const name = ref(s?.name ?? ''), path = ref(s?.path ?? '/mnt/user');
const readOnly = ref(s?.readOnly ?? false), indexed = ref(s?.indexed ?? true), skipHidden = ref(s?.skipHidden !== false);
const exclude = ref((s?.exclude ?? []).join('\n'));
const nameEl = ref<HTMLInputElement | null>(null), pathEl = ref<InstanceType<typeof PathField> | null>(null);

async function save() {
  const n = name.value.trim(), p = path.value.trim().replace(/\/+$/, '') || '/';
  if (!n) return fail(c('sources.needName'), nameEl.value);
  if (props.all.some((x) => x !== s && x.name.toLowerCase() === n.toLowerCase())) return fail(c('nameTaken'), nameEl.value);
  if (!(await exists(p))) return fail(c('noFolder'), pathEl.value?.input);
  if (props.all.some((x) => x !== s && x.path === p)) return fail(c('sources.pathTaken'), pathEl.value?.input);
  const next: Source = {
    ...(s ?? { files: 0, size: 0 }), name: n, path: p, readOnly: readOnly.value, indexed: indexed.value, skipHidden: skipHidden.value,
    exclude: exclude.value.split('\n').map((x) => x.trim()).filter(Boolean),
  };
  const startIndex = next.indexed && (!s || !s.indexed || s.path !== p);
  if (!next.indexed || (s && s.path !== p)) Object.assign(next, { files: 0, size: 0 });
  if (startIndex) next.indexed = false;
  await admin.saveSources(s ? props.all.map((x) => (x === s ? next : x)) : [...props.all, next]);
  props.win.close();
  toast(s ? c('saved') : c('added', { name: n }));
  props.saved(startIndex ? p : null);
}

onMounted(() => setFoot(props.win, [cancelBtn(), { label: s ? c('save') : c('add'), g: s ? 'IconSave' : 'IconAdd', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <div class="well">
    <Frow :label="c('sources.name')"><input ref="nameEl" v-model="name" class="field m" :placeholder="c('sources.namePh')"></Frow>
    <Frow :label="c('sources.folder')"><PathField ref="pathEl" v-model="path" /></Frow>
  </div>
  <div class="well">
    <Frow :label="c('sources.readOnly')" :tip="c('sources.readOnlyTip')"><Sw v-model="readOnly" /></Frow>
    <Frow :label="c('sources.indexIt')" :tip="c('sources.indexItTip')"><Sw v-model="indexed" /></Frow>
    <Frow :label="c('sources.skipHidden')" :tip="c('sources.skipHiddenTip')"><Sw v-model="skipHidden" /></Frow>
  </div>
  <div class="well">
    <div class="eyebrow">{{ c('sources.exclude') }} <Info :tip="c('sources.excludeTip')" /></div>
    <textarea v-model="exclude" class="field" rows="3" spellcheck="false" placeholder="*.tmp"></textarea>
  </div>
</template>
