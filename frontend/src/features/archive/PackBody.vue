<script setup lang="ts">
// Verpacken: name, format, compression and password of the archive, and the folder it lands in.
import { pathField } from '@/ui/picker';
import { FieldRow, PasswordField, PathField, Pill, Selector, SubCard, TextField } from '@glimstone/vue';
import { computed, watch, watchEffect } from 'vue';
import { fmtSize } from '@/core/format';
import type { Entry } from '@/files/entries';
import { join } from '@/files/paths';
import { t } from '@/i18n';
import FinalPath from '@/ui/FinalPath.vue';
import { eyeWords, plural, segItems } from '@/ui/kit';
import { clean, folderState, freeName, namesIn } from '@/ui/tools';

export interface PackState { items: Entry[]; name: string; format: string; level: string; password: string; dest: string; formats: string[]; file: string }
const props = defineProps<{ st: PackState }>();

const total = computed(() => props.st.items.reduce((a, k) => a + (k.size || 0), 0));
const formats = computed(() => props.st.formats.map((f) => [f, f] as const));
const levels = computed(() => ['0', '3', '6', '9'].map((l) => [l, t(`archive.levels.${l}`)] as const));
const tar = computed(() => props.st.format.startsWith('tar'));
watch(tar, (no) => { if (no) props.st.password = ''; });

const ok = folderState(() => props.st.dest);
const wanted = computed(() => `${props.st.name.trim()}.${props.st.format}`);
watchEffect(() => { props.st.file = ok.value && props.st.name.trim() ? freeName(namesIn(clean(props.st.dest)), wanted.value) : ''; });
</script>

<template>
  <SubCard :title="`${plural(st.items.length, 'ui.objects')}, ${fmtSize(total) || '0 B'}`" class="bk-form">
    <FieldRow :label="t('archive.name')"><span class="bk-namef"><TextField id="k-name" v-model="st.name" spellcheck="false" autocomplete="off" width="grow" /><Pill id="k-ext" class="mono">.{{ st.format }}</Pill></span></FieldRow>
    <FieldRow :label="t('archive.format')" :hint="t('archive.formatTip')"><Selector id="k-fmt" v-model="st.format" :items="segItems(formats)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="t('archive.level')" :hint="t('archive.levelTip')"><Selector id="k-level" v-model="st.level" :items="segItems(levels)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="t('archive.password')" :hint="t('archive.passwordTip')"><PasswordField id="k-pw" v-model="st.password" width="grow" :placeholder="t('archive.noPassword')" :disabled="tar" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
  </SubCard>
  <SubCard :title="t('archive.target')" class="bk-form">
    <FieldRow :label="t('archive.folder')"><PathField v-model="st.dest" v-bind="pathField('k-dest')" /></FieldRow>
    <FieldRow :label="t('archive.savedAs')">
      <span id="k-final" class="bk-finalw">
        <Pill v-if="ok === false" tone="fail">{{ t('ui.noFolder') }}</Pill>
        <Pill v-else-if="!st.name.trim()" tone="fail">{{ t('ui.noName') }}</Pill>
        <template v-else-if="st.file"><FinalPath :path="join(clean(st.dest), st.file)" /><Pill v-if="st.file !== wanted" :data-tip="t('archive.takenTip')" tone="warn">{{ t('ui.taken') }}</Pill></template>
      </span>
    </FieldRow>
  </SubCard>
</template>
