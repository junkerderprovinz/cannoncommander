<script setup lang="ts">
import { pathField } from '@/ui/picker';
import { FieldRow, PathField, SubCard, TextArea, TextField, Toggle } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { admin, exists, type Source } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { attempt, c, fail, stepFoot } from '@/features/config/shared';

const props = defineProps<{ win: WinEntry; source: Source | null; all: Source[]; saved: () => void }>();
const s = props.source;
const name = ref(s?.name ?? ''), path = ref(s?.path ?? '/mnt/user');
const readOnly = ref(s?.readOnly ?? false), indexed = ref(s?.indexed ?? true), skipHidden = ref(s?.skipHidden !== false);
const exclude = ref((s?.exclude ?? []).join('\n'));
const nameEl = ref<InstanceType<typeof TextField> | null>(null), pathEl = ref<InstanceType<typeof PathField> | null>(null);
const at = (sel: string) => props.win.el?.querySelector(sel) ?? null;

// The index takes a name, a path below the source, or a name with one star at its start or end.
function badPattern(p: string) {
  if (!p || p === '*') return c('sources.patternAll');
  if (p.includes('/')) return p.includes('*') ? c('sources.patternPath') : null;
  return p.slice(1, -1).includes('*') ? c('sources.patternStar') : null;
}

async function save() {
  const n = name.value.trim(), p = path.value.trim().replace(/\/+$/, '') || '/';
  if (!n) return fail(c('sources.needName'), nameEl.value?.input);
  if (props.all.some((x) => x !== s && x.name.toLowerCase() === n.toLowerCase())) return fail(c('nameTaken'), nameEl.value?.input);
  // The server checks every folder on save, so only a new one is looked up here first.
  if (p !== s?.path && !(await exists(p))) return fail(c('noFolder'), pathEl.value?.input);
  if (props.all.some((x) => x !== s && x.path === p)) return fail(c('sources.pathTaken'), pathEl.value?.input);
  const patterns = exclude.value.split('\n').map((x) => x.trim().replace(/^\/+|\/+$/g, '')).filter(Boolean);
  for (const pat of patterns) {
    const why = badPattern(pat);
    if (why) return fail(why, at('#a-sex'));
  }
  const next: Source = { name: n, path: p, readOnly: readOnly.value, indexed: indexed.value, skipHidden: skipHidden.value, exclude: patterns };
  const done = await attempt(admin.saveSources(s ? props.all.map((x) => (x === s ? next : x)) : [...props.all, next]), at('#a-sex'));
  if (!done) return;
  props.win.close();
  toast(done.value.restartNeeded ? c('sources.savedRestart') : s ? c('saved') : c('added', { name: n }));
  props.saved();
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: s ? c('save') : c('add'), g: s ? 'IconSave' : 'IconAdd', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <SubCard>
    <FieldRow :label="c('sources.name')"><TextField id="a-sname" ref="nameEl" v-model="name" :placeholder="c('sources.namePh')" width="medium" /></FieldRow>
    <FieldRow :label="c('sources.folder')"><PathField ref="pathEl" v-model="path" v-bind="pathField('a-spath')" /></FieldRow>
  </SubCard>
  <SubCard>
    <FieldRow :label="c('sources.readOnly')" :hint="c('sources.readOnlyTip')"><Toggle v-model="readOnly" /></FieldRow>
    <FieldRow :label="c('sources.indexIt')" :hint="c('sources.indexItTip')"><Toggle v-model="indexed" /></FieldRow>
    <FieldRow :label="c('sources.skipHidden')" :hint="c('sources.skipHiddenTip')"><Toggle v-model="skipHidden" /></FieldRow>
  </SubCard>
  <SubCard :title="c('sources.exclude')" :hint="c('sources.excludeTip')">
    <TextArea id="a-sex" v-model="exclude" rows="3" spellcheck="false" placeholder="*.tmp" />
  </SubCard>
</template>
