<script setup lang="ts">
// Herunterladen of several entries or a folder: packed by the server while it sends, as zip or tar.gz.
import { FieldRow, Pill, Selector, SubCard, Table, TextField } from '@glimstone/vue';
import { computed, watch } from 'vue';
import { fmtSize } from '@/core/format';
import type { Entry } from '@/files/entries';
import { t } from '@/i18n';
import { plural, segItems } from '@/ui/kit';

export interface DownloadState { items: Entry[]; base: string; format: 'zip' | 'tgz'; name: string }
const props = defineProps<{ st: DownloadState }>();
const formats = [['zip', 'ZIP'], ['tgz', 'TAR.GZ']] as const;
const total = computed(() => props.st.items.reduce((a, k) => a + (k.size || 0), 0));
watch(() => props.st.format, (f) => { props.st.name = `${props.st.base}.${f === 'zip' ? 'zip' : 'tar.gz'}`; });
</script>

<template>
  <SubCard :title="`${plural(st.items.length, 'ui.objects')}, ${fmtSize(total)}`">
    <Table :label="t('share.format')" :columns="[{ key: 'name' }, { key: 'size', end: true }]" :rows="st.items.slice(0, 6).map((k) => ({ name: k.name, size: fmtSize(k.size) || '0 B' }))" :more="st.items.length > 6 ? t('share.more', { n: st.items.length - 6 }) : undefined" />
  </SubCard>
  <SubCard :title="t('share.archive')">
    <FieldRow :label="t('share.format')" :hint="t('share.formatTip')"><Selector id="dl-fmt" v-model="st.format" :items="segItems(formats)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="t('share.fileName')"><TextField id="dl-name" v-model="st.name" width="medium" /></FieldRow>
    <FieldRow :label="t('share.savesIn')" :hint="t('share.savesInTip')"><Pill>{{ t('share.browserFolder') }}</Pill></FieldRow>
  </SubCard>
</template>
