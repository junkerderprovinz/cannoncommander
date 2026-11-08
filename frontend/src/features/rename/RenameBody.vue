<script setup lang="ts">
// Mehrfach umbenennen: the pattern on the left, find and replace on the right, the preview of every name below.
import { Select } from '@glimstone/vue/Select';
import { FieldRow, FieldRowGroup, IconButton, Pill, SubCard, TextField, Toggle } from '@glimstone/vue';
import { computed, nextTick, ref, watch } from 'vue';
import type { RenameMask, RenamePreset, RenameRow } from '@/api/tools';
import { G, glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { plural, pressPrimary, selectOptions } from '@/ui/kit';
import { BUILT_IN, CASES } from './presets';

export interface RenameState { o: RenameMask; rows: RenameRow[]; bad: boolean; count: number; own: RenamePreset[]; preset: string }
const props = defineProps<{ st: RenameState; savePreset: () => void; dropPreset: () => void }>();

const TOKENS = ['N', 'E', 'C', 'D', 'P'];
const META = ['A', 'K', 'I', 'T', 'R', 'S', 'F'];
const chip = (id: string) => (/[SF]/.test(id) ? `[${id}2]` : `[${id}]`);
const label = (id: string) => t(`rename.tokens.${id}.label`);

const presets = computed(() => [
  ['', t('rename.noPreset')] as const,
  ...BUILT_IN.map((p) => ['b:' + p.key, t(`rename.builtIn.${p.key}`)] as const),
  ...props.st.own.map((p) => ['own:' + p.name, p.name, 'IconStar'] as const),
]);
const caseOptions = computed(() => CASES.map((c) => [c, t(`rename.cases.${c}`)] as const));
watch(() => props.st.preset, (v) => {
  const p = v.startsWith('b:') ? BUILT_IN.find((x) => 'b:' + x.key === v)?.options : props.st.own.find((x) => 'own:' + x.name === v)?.options;
  if (p) Object.assign(props.st.o, p);
});

const fields = { name: ref<InstanceType<typeof TextField> | null>(null), ext: ref<InstanceType<typeof TextField> | null>(null) };
let focused = 'name' as 'name' | 'ext';
async function insert(token: string) {
  const f = fields[focused].value!.input!, a = f.selectionStart ?? f.value.length, z = f.selectionEnd ?? a;
  props.st.o[focused] = f.value.slice(0, a) + token + f.value.slice(z);
  await nextTick();
  f.focus();
  f.setSelectionRange(a + token.length, a + token.length);
}
const num = (v: string, fallback: number) => parseInt(v, 10) || fallback;

const changed = computed(() => props.st.rows.filter((r) => r.new !== r.old).length);
const missingTip = (r: RenameRow) => t('rename.withoutTip', { names: r.missing!.map(label).join(', ') });
</script>

<template>
  <div class="bk-grid2 bk-top" @keydown.enter="pressPrimary">
    <SubCard :title="t('rename.mask')" class="bk-form">
      <FieldRow :label="t('rename.preset')" :hint="t('rename.presetTip')">
        <span id="m-presets" class="bk-range"><Select id="m-preset" v-model="st.preset" :trigger-attrs="{ 'data-dd': JSON.stringify(presets) }" :options="selectOptions(presets)" />
          <IconButton id="m-psave" :tip="t('rename.savePreset')" :glyph="glyphComponent('IconSave')" @click="savePreset" />
          <IconButton id="m-pdel" :tip="t('rename.dropPreset')" :glyph="glyphComponent('IconTrash')" :disabled="!st.preset.startsWith('own:')" @click="dropPreset" /></span>
      </FieldRow>
      <FieldRowGroup>
        <FieldRow :label="t('rename.name')"><TextField id="m-name" :ref="fields.name" v-model="st.o.name" spellcheck="false" autocomplete="off" width="grow" mono @focus="focused = 'name'" /></FieldRow>
        <FieldRow :label="t('rename.ext')"><TextField id="m-ext" :ref="fields.ext" v-model="st.o.ext" spellcheck="false" autocomplete="off" width="grow" mono @focus="focused = 'ext'" /></FieldRow>
      </FieldRowGroup>
      <div class="bk-tokens"><button v-for="id in TOKENS" :key="id" class="bk-chip" type="button" :data-bk-tok="`[${id}]`" :data-tip="t(`rename.tokens.${id}.tip`)" @click="insert(`[${id}]`)"><b>[{{ id }}]</b>{{ label(id) }}</button></div>
      <div class="bk-tokens bk-meta"><span class="muted">{{ t('rename.fromMeta') }}</span><button v-for="id in META" :key="id" class="bk-chip" type="button" :data-bk-tok="chip(id)" :data-tip="t(`rename.tokens.${id}.tip`)" @click="insert(chip(id))"><b>{{ chip(id) }}</b>{{ label(id) }}</button></div>
      <FieldRow :label="t('rename.counter')">
        <span class="bk-range"><span class="muted">{{ t('rename.from') }}</span><TextField id="m-start" :model-value="String(st.o.start)" class="glim-num" inputmode="numeric" width="short" @input="st.o.start = num(($event.target as HTMLInputElement).value, 0)" /><span class="muted">{{ t('rename.step') }}</span><TextField id="m-step" :model-value="String(st.o.step)" class="glim-num" inputmode="numeric" width="short" @input="st.o.step = num(($event.target as HTMLInputElement).value, 1)" /></span>
      </FieldRow>
    </SubCard>
    <SubCard :title="t('rename.findReplace')" class="bk-form">
      <FieldRowGroup>
        <FieldRow :label="t('rename.find')"><TextField id="m-find" v-model="st.o.find" spellcheck="false" autocomplete="off" width="grow" mono /></FieldRow>
        <FieldRow :label="t('rename.replace')"><TextField id="m-repl" v-model="st.o.repl" spellcheck="false" autocomplete="off" width="grow" mono /></FieldRow>
        <FieldRow :label="t('rename.regex')" :hint="t('rename.regexTip')"><Toggle id="m-regex" v-model="st.o.regex" /></FieldRow>
        <FieldRow :label="t('rename.case')"><Select id="m-case" v-model="st.o.case" :trigger-attrs="{ 'data-val': st.o.case }" :options="selectOptions(caseOptions)" /></FieldRow>
      </FieldRowGroup>
    </SubCard>
  </div>
  <SubCard class="bk-fill"><div class="bk-tools"><span id="m-count" class="glim-eyebrow">{{ t('rename.count', { objects: plural(st.count, 'ui.objects'), changes: plural(changed, 'rename.changes') }) }}</span></div>
    <div class="bk-mhead"><span>{{ t('rename.before') }}</span><span></span><span>{{ t('rename.after') }}</span><span></span></div>
    <div id="m-list" class="bk-list">
      <div v-for="r in st.rows" :key="r.path" class="bk-mrow" :class="{ same: r.new === r.old }">
        <span class="bk-name mono">{{ r.old }}</span><span class="bk-arrow sm"><G n="IconForward" /></span>
        <span class="bk-name mono" :class="{ new: !r.error }">{{ r.new || ' ' }}</span>
        <Pill v-if="r.error" tone="fail">{{ t(`rename.errors.${r.error}`) }}</Pill>
        <Pill v-else-if="r.missing?.length" :data-tip="missingTip(r)">{{ t('rename.without', { name: label(r.missing[0]!) }) }}</Pill>
        <span v-else></span>
      </div>
    </div>
  </SubCard>
</template>
