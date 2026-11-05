<script setup lang="ts">
// Zusammenfügen: every part beside the picked one, gaps among them, and the whole file they make.
import { pathField } from '@/ui/picker';
import { FieldRow, PathField, Pill, TextField, Toggle } from '@glimstone/vue';
import { computed, watchEffect } from 'vue';
import { fmtSize } from '@/core/format';
import { join } from '@/files/paths';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import FinalPath from '@/ui/FinalPath.vue';
import { plural } from '@/ui/kit';
import { clean, folderState, freeName, namesIn } from '@/ui/tools';

export interface CombineState { dir: string; base: string; name: string; dest: string; check: boolean; drop: boolean; file: string; ok: boolean }
const props = defineProps<{ st: CombineState }>();
const listing = useListing();

const items = computed(() => listing.lists.get(props.st.dir)?.items ?? []);
const parts = computed(() => items.value.filter((k) => !k.dir && k.name.startsWith(props.st.base + '.') && /^\d{3,}$/.test(k.name.slice(props.st.base.length + 1)))
  .sort((a, b) => a.name.localeCompare(b.name, 'de', { numeric: true })));
const missing = computed(() => {
  const nums = parts.value.map((p) => +p.name.slice(props.st.base.length + 1));
  return nums.length ? Array.from({ length: Math.max(...nums) }, (_, i) => i + 1).filter((i) => !nums.includes(i)) : [];
});
const total = computed(() => parts.value.reduce((a, p) => a + p.size, 0));
const sum = computed(() => items.value.find((k) => k.name === props.st.base + '.sha256'));
const ok = folderState(() => props.st.dest);
watchEffect(() => {
  const st = props.st, name = st.name.trim();
  if (!sum.value) st.check = false;
  st.file = ok.value && name ? freeName(namesIn(clean(st.dest)), name) : '';
  st.ok = !!st.file && !missing.value.length && parts.value.length >= 2;
});
</script>

<template>
  <div class="well glim-well"><div class="eyebrow">{{ t('checksum.partsOf', { parts: plural(parts.length, 'checksum.partsWord'), size: fmtSize(total) }) }}</div>
    <table class="tbl">
      <tr v-for="p in parts" :key="p.name"><td class="mono">{{ p.name }}</td><td class="glim-num r">{{ fmtSize(p.size) }}</td></tr>
      <tr v-if="missing.length"><td colspan="2"><Pill tone="fail">{{ t('checksum.missingParts', { list: missing.map((i) => '.' + String(i).padStart(3, '0')).join(', ') }) }}</Pill></td></tr>
    </table>
  </div>
  <div class="well glim-well bk-form"><div class="eyebrow">{{ t('checksum.result') }}</div>
    <FieldRow :label="t('checksum.name')"><TextField id="j-name" v-model="st.name" spellcheck="false" autocomplete="off" width="grow" mono /></FieldRow>
    <FieldRow :label="t('checksum.folder')"><PathField v-model="st.dest" v-bind="pathField('j-dest')" /></FieldRow>
    <FieldRow :label="t('checksum.checkAfter')" :hint="sum ? t('checksum.checkWith', { name: sum.name }) : t('checksum.checkNone')"><Toggle id="j-check" v-model="st.check" :disabled="!sum" /></FieldRow>
    <FieldRow :label="t('checksum.dropParts')" :hint="t('checksum.dropPartsTip')"><Toggle id="j-drop" v-model="st.drop" /></FieldRow>
    <FieldRow :label="t('checksum.savedAs')">
      <span id="j-final" class="bk-finalw">
        <Pill v-if="ok === false" tone="fail">{{ t('ui.noFolder') }}</Pill>
        <Pill v-else-if="!st.name.trim()" tone="fail">{{ t('ui.noName') }}</Pill>
        <template v-else-if="st.file"><FinalPath :path="join(clean(st.dest), st.file)" /><Pill v-if="st.file !== st.name.trim()" :data-tip="t('checksum.takenTip')" tone="warn">{{ t('ui.taken') }}</Pill></template>
      </span>
    </FieldRow>
  </div>
</template>
