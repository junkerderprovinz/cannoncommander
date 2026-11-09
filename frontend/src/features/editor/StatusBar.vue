<script setup lang="ts">
// The status bar under the text, as in Notepad++: where the caret is and how much is selected, then how the file is
// written, each a picker that changes it. An encoding is converted on save, or the file read again in another one
// when the guess was wrong.
import { Pill, Toggle } from '@glimstone/vue';
import { Select } from '@glimstone/vue/Select';
import { computed } from 'vue';
import { plural, t } from '@/i18n';
import { type Doc, type Eol, ENCODINGS, EOL_NAMES, EOLS, type Indent } from './docs';
import { LANGUAGES } from './languages';

const props = defineProps<{ doc: Doc; line: number; col: number; chosen: number; wrap: boolean }>();
const emit = defineEmits<{ lang: [id: string]; convert: [encoding: string]; reopen: [encoding: string]; eol: [eol: Eol]; indent: [indent: Indent]; wrap: [on: boolean]; goto: [] }>();

const PLAIN = 'plain';
const languages = computed(() => [
  { entries: [{ value: PLAIN, label: t('editor.plain') }] },
  { entries: [...LANGUAGES].sort((a, b) => a.name.localeCompare(b.name)).map((l) => ({ value: l.id, label: l.name })) },
]);
const encodings = computed(() => [
  { heading: t('editor.saveAs'), entries: ENCODINGS.map(([id, name]) => ({ value: 'to:' + id, label: name })) },
  { entries: [{ label: t('editor.reopenAs'), options: ENCODINGS.map(([id, name]) => ({ value: 'as:' + id, label: name })) }] },
]);
const eols = computed(() => EOLS.map((e) => ({ value: e, label: `${t(`editor.eol.${e}`)} (${EOL_NAMES[e]})` })));
const indents = computed(() => [
  { heading: t('editor.indentSpaces'), entries: [2, 4, 8].map((n) => ({ value: `spaces:${n}`, label: t('editor.spaces', { n }) })) },
  { heading: t('editor.indentTabs'), entries: [2, 4, 8].map((n) => ({ value: `tabs:${n}`, label: t('editor.tabWidth', { n }) })) },
]);

function encoding(v: string | null) {
  if (!v) return;
  const [how, id] = v.split(':') as ['to' | 'as', string];
  if (how === 'to') emit('convert', id);
  else emit('reopen', id);
}
function indent(v: string | null) {
  if (!v) return;
  const [kind, n] = v.split(':');
  emit('indent', { tabs: kind === 'tabs', width: Number(n) });
}
</script>

<template>
  <div class="ce-status glim-num" role="status">
    <button type="button" class="ce-pos" :title="t('editor.cmd.gotoLine')" @click="emit('goto')">
      {{ t('editor.pos', { line, col }) }}<template v-if="chosen">, {{ plural(chosen, 'editor.marked') }}</template>
    </button>
    <span class="grow"></span>
    <Pill v-if="doc.cut" class="ce-flag">{{ t('editor.onlyStart') }}</Pill>
    <Pill v-else-if="doc.readOnly" class="ce-flag">{{ t('editor.readOnly') }}</Pill>
    <Select id="ce-indent" :model-value="`${doc.indent.tabs ? 'tabs' : 'spaces'}:${doc.indent.width}`" :groups="indents" :tip="t('editor.indent')" size="inset" width="hug" @update:model-value="indent" />
    <Select id="ce-eol" :model-value="doc.eol" :options="eols" :tip="t('editor.eolTip')" size="inset" width="hug" :disabled="doc.readOnly" @update:model-value="(v) => v && emit('eol', v as Eol)" />
    <Select id="ce-enc" :model-value="'to:' + doc.encoding" :groups="encodings" :tip="t('editor.encoding')" size="inset" width="hug" @update:model-value="encoding" />
    <Select id="ce-lang" :model-value="doc.lang || PLAIN" :groups="languages" :tip="t('editor.language')" size="inset" width="hug" @update:model-value="(v) => v && emit('lang', v === PLAIN ? '' : v)" />
    <span class="cv-swl ce-wrapsw">{{ t('editor.wrap') }} <Toggle id="ce-wrap" :model-value="wrap" @update:model-value="emit('wrap', $event)" /></span>
  </div>
</template>
