<script setup lang="ts">
// Entpacken: the archives, or what is picked inside one, and where it all lands.
import { pathField } from '@/ui/picker';
import { FieldRow, FieldRowGroup, PasswordField, PathField, Pill, Selector, SubCard, Table, Toggle } from '@glimstone/vue';
import { computed, watchEffect } from 'vue';
import { fmtSize } from '@/core/format';
import type { Entry } from '@/files/entries';
import { join } from '@/files/paths';
import { t } from '@/i18n';
import FinalPath from '@/ui/FinalPath.vue';
import { eyeWords, plural, segItems } from '@/ui/kit';
import { clean, folderState } from '@/ui/tools';
import { stripArchive } from './names';

export interface UnpackState {
  inside: boolean; items: Entry[]; archive: string; dest: string; own: boolean; clash: 'skip' | 'both' | 'replace';
  password: string; locked: boolean; ok: boolean; size: number | null;
}
const props = defineProps<{ st: UnpackState }>();
const clashes = computed(() => [['skip', t('archive.skip'), 'IconKeepRight'], ['both', t('archive.both'), 'IconKeepBoth'], ['replace', t('archive.replace'), 'IconKeepLeft']] as const);
const ok = folderState(() => props.st.dest);
watchEffect(() => { props.st.ok = !!ok.value; });
const head = computed(() => (props.st.inside ? t('archive.fromArchive', { objects: plural(props.st.items.length, 'ui.objects'), name: props.st.archive }) : plural(props.st.items.length, 'archive.archives')));
</script>

<template>
  <SubCard :title="head">
    <Table :label="t('archive.folder')" :columns="[{ key: 'name' }, { key: 'size', end: true }]" :rows="st.items.slice(0, 5).map((k) => ({ name: k.name, size: fmtSize(k.size) }))" :more="st.items.length > 5 ? plural(st.items.length - 5, 'archive.more') : undefined" />
  </SubCard>
  <SubCard :title="t('archive.target')" class="bk-form">
    <FieldRowGroup>
      <FieldRow :label="t('archive.folder')"><PathField v-model="st.dest" v-bind="pathField('x-dest')" /></FieldRow>
      <FieldRow :label="t('archive.own')" :hint="t('archive.ownTip')"><Toggle id="x-own" v-model="st.own" /></FieldRow>
      <FieldRow :label="t('archive.clash')" :hint="t('archive.clashTip')"><Selector id="x-clash" v-model="st.clash" :items="segItems(clashes)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
      <FieldRow v-if="st.locked" :label="t('archive.unpackPassword')" :hint="t('archive.unpackPasswordTip')"><PasswordField id="x-pw" v-model="st.password" width="grow" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
    </FieldRowGroup>
    <FieldRow :label="t('archive.lands')">
      <span id="x-final" class="bk-finalw">
        <Pill v-if="ok === false" tone="fail">{{ t('ui.noFolder') }}</Pill>
        <template v-else-if="st.own"><FinalPath :path="join(clean(st.dest), stripArchive(st.inside ? st.archive : st.items[0]!.name))" /><Pill v-if="!st.inside && st.items.length > 1">{{ plural(st.items.length - 1, 'archive.more') }}</Pill></template>
        <FinalPath v-else :path="clean(st.dest)" />
      </span>
    </FieldRow>
  </SubCard>
</template>
