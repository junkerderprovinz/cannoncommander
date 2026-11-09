<script setup lang="ts">
// The program F4 opens text files in: the built-in editor, a program the computer offers for text files, or another
// picked in the system's dialog. Linux has no such dialog, so there every program of the desktop's menus is offered.
import { FieldRow } from '@glimstone/vue';
import { Select } from '@glimstone/vue/Select';
import { computed, onMounted, ref } from 'vue';
import { pictureComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import type { App } from '@/features/openwith/apps';
import { t } from '@/i18n';
import { editorProgram, type EditorProgram, pickProgram, setEditorProgram, textPrograms } from './program';

const BUILT_IN = 'builtin', OTHER = 'other', EVERY = 'every:';
const apps = ref<App[]>([]);
const every = ref<App[]>([]);
const dialog = ref(true);
const busy = ref(false);

const keyOf = (p: EditorProgram | null) => (p ? `${p.kind}:${p.every ? EVERY : ''}${p.id}` : BUILT_IN);
const value = computed(() => keyOf(editorProgram()));
const glyphOf = (icon?: string) => (icon ? pictureComponent(icon) : undefined);

onMounted(async () => {
  try {
    const got = await textPrograms();
    apps.value = got.apps;
    dialog.value = got.dialog;
    if (!got.dialog) every.value = (await textPrograms(true)).apps;
  } catch {
    // Without the list the built-in editor stays on offer.
  }
});

const groups = computed(() => {
  const p = editorProgram();
  const offered = apps.value.map((a) => ({ value: `app:${a.id}`, label: a.name, glyph: glyphOf(a.icon) }));
  // A program picked earlier stays in the list, also when the computer stops naming it for text files.
  const own = p && !offered.some((o) => o.value === keyOf(p)) ? [{ value: keyOf(p), label: p.name, glyph: glyphOf(p.icon) }] : [];
  const other = dialog.value
    ? [{ value: OTHER, label: t('editor.program.other') }]
    : [{ label: t('editor.program.other'), options: every.value.map((a) => ({ value: `app:${EVERY}${a.id}`, label: a.name, glyph: glyphOf(a.icon) })) }];
  return [
    { entries: [{ value: BUILT_IN, label: t('editor.program.builtIn') }] },
    ...(offered.length || own.length ? [{ heading: t('editor.program.offered'), entries: [...offered, ...own] }] : []),
    { entries: other },
  ];
});

function take(p: EditorProgram | null) {
  setEditorProgram(p);
  toast(p ? t('editor.program.set', { name: p.name }) : t('editor.program.builtInSet'));
}

async function choose(v: string | null) {
  if (!v || v === value.value) return;
  if (v === BUILT_IN) return take(null);
  if (v === OTHER) {
    busy.value = true;
    try {
      const p = await pickProgram();
      if (p) take(p);
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'warn');
    } finally {
      busy.value = false;
    }
    return;
  }
  const fromEvery = v.startsWith('app:' + EVERY), id = v.slice(fromEvery ? 4 + EVERY.length : 4);
  const a = (fromEvery ? every.value : apps.value).find((x) => x.id === id);
  if (a) take({ kind: 'app', id: a.id, name: a.name, icon: a.icon, ...(fromEvery ? { every: true } : {}) });
}
</script>

<template>
  <FieldRow :label="t('editor.program.label')" :hint="t('editor.program.tip')">
    <Select id="d-editor" :model-value="value" :groups="groups" :busy="busy" @update:model-value="choose" />
  </FieldRow>
</template>
