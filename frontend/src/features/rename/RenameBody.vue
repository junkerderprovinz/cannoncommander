<script setup lang="ts">
// Mehrfach umbenennen: the pattern on the left, find and replace on the right, the preview of every name below.
import { computed, nextTick, ref, watch } from 'vue';
import type { RenameMask, RenamePreset, RenameRow } from '@/api/tools';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import Dropdown from '@/ui/Dropdown.vue';
import Frow from '@/ui/Frow.vue';
import { plural, pressPrimary } from '@/ui/kit';
import Sw from '@/ui/Sw.vue';
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

const fields = { name: ref<HTMLInputElement | null>(null), ext: ref<HTMLInputElement | null>(null) };
let focused = 'name' as 'name' | 'ext';
async function insert(token: string) {
  const f = fields[focused].value!, a = f.selectionStart ?? f.value.length, z = f.selectionEnd ?? a;
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
    <div class="well glim-well bk-form"><div class="eyebrow">{{ t('rename.mask') }}</div>
      <Frow :label="t('rename.preset')" :tip="t('rename.presetTip')">
        <span id="m-presets" class="bk-range"><Dropdown id="m-preset" v-model="st.preset" :options="presets" size="grow" />
          <button id="m-psave" class="btn icon" type="button" :data-tip="t('rename.savePreset')" :aria-label="t('rename.savePreset')" @click="savePreset"><G n="IconSave" /></button>
          <button id="m-pdel" class="btn icon" type="button" :data-tip="t('rename.dropPreset')" :aria-label="t('rename.dropPreset')" :disabled="!st.preset.startsWith('own:')" @click="dropPreset"><G n="IconTrash" /></button></span>
      </Frow>
      <Frow :label="t('rename.name')"><input id="m-name" :ref="fields.name" v-model="st.o.name" class="glim-input grow glim-input--mono" spellcheck="false" autocomplete="off" @focus="focused = 'name'"></Frow>
      <Frow :label="t('rename.ext')"><input id="m-ext" :ref="fields.ext" v-model="st.o.ext" class="glim-input grow glim-input--mono" spellcheck="false" autocomplete="off" @focus="focused = 'ext'"></Frow>
      <div class="bk-tokens"><button v-for="id in TOKENS" :key="id" class="bk-chip" type="button" :data-bk-tok="`[${id}]`" :data-tip="t(`rename.tokens.${id}.tip`)" @click="insert(`[${id}]`)"><b>[{{ id }}]</b>{{ label(id) }}</button></div>
      <div class="bk-tokens bk-meta"><span class="muted">{{ t('rename.fromMeta') }}</span><button v-for="id in META" :key="id" class="bk-chip" type="button" :data-bk-tok="chip(id)" :data-tip="t(`rename.tokens.${id}.tip`)" @click="insert(chip(id))"><b>{{ chip(id) }}</b>{{ label(id) }}</button></div>
      <Frow :label="t('rename.counter')">
        <span class="bk-range"><span class="muted">{{ t('rename.from') }}</span><input id="m-start" class="glim-input s glim-num" :value="st.o.start" inputmode="numeric" @input="st.o.start = num(($event.target as HTMLInputElement).value, 0)"><span class="muted">{{ t('rename.step') }}</span><input id="m-step" class="glim-input s glim-num" :value="st.o.step" inputmode="numeric" @input="st.o.step = num(($event.target as HTMLInputElement).value, 1)"></span>
      </Frow>
    </div>
    <div class="well glim-well bk-form"><div class="eyebrow">{{ t('rename.findReplace') }}</div>
      <Frow :label="t('rename.find')"><input id="m-find" v-model="st.o.find" class="glim-input grow glim-input--mono" spellcheck="false" autocomplete="off"></Frow>
      <Frow :label="t('rename.replace')"><input id="m-repl" v-model="st.o.repl" class="glim-input grow glim-input--mono" spellcheck="false" autocomplete="off"></Frow>
      <Frow :label="t('rename.regex')" :tip="t('rename.regexTip')"><Sw id="m-regex" v-model="st.o.regex" /></Frow>
      <Frow :label="t('rename.case')"><Dropdown id="m-case" v-model="st.o.case" :options="caseOptions" size="grow" /></Frow>
    </div>
  </div>
  <div class="well glim-well bk-fill"><div class="bk-tools"><span id="m-count" class="eyebrow">{{ t('rename.count', { objects: plural(st.count, 'ui.objects'), changes: plural(changed, 'rename.changes') }) }}</span></div>
    <div class="bk-mhead"><span>{{ t('rename.before') }}</span><span></span><span>{{ t('rename.after') }}</span><span></span></div>
    <div id="m-list" class="bk-list">
      <div v-for="r in st.rows" :key="r.path" class="bk-mrow" :class="{ same: r.new === r.old }">
        <span class="bk-name mono">{{ r.old }}</span><span class="bk-arrow sm"><G n="IconForward" /></span>
        <span class="bk-name mono" :class="{ new: !r.error }">{{ r.new || ' ' }}</span>
        <span v-if="r.error" class="pill fault">{{ t(`rename.errors.${r.error}`) }}</span>
        <span v-else-if="r.missing?.length" class="pill" :data-tip="missingTip(r)">{{ t('rename.without', { name: label(r.missing[0]!) }) }}</span>
        <span v-else></span>
      </div>
    </div>
  </div>
</template>
