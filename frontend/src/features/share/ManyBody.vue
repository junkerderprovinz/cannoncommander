<script setup lang="ts">
// Sammelfreigabe: entries from several folders under one link, which the other side sees as one folder.
import { FieldRow, SubCard, Table, TextField } from '@glimstone/vue';
import { computed } from 'vue';
import { fmtSize } from '@/core/format';
import { parent } from '@/files/paths';
import { t } from '@/i18n';
import { plural } from '@/ui/kit';
import type { Collected } from './basket';
import type { LinkForm } from './link';
import LinkOptions from './LinkOptions.vue';

const props = defineProps<{ items: Collected[]; f: LinkForm; st: { title: string } }>();
const folders = computed(() => new Set(props.items.map((x) => parent(x.path))).size);
const size = computed(() => fmtSize(props.items.reduce((a, x) => a + x.size, 0)) || '0 B');
</script>

<template>
  <SubCard :title="`${t('share.fromFolders', { objects: plural(items.length, 'ui.objects'), folders: plural(folders, 'share.folders') })}, ${size}`">
    <Table :label="t('share.folderName')" :columns="[{ key: 'name' }, { key: 'from', tone: 'muted' }]" :rows="items.slice(0, 6).map((x) => ({ name: x.name, from: parent(x.path) }))" :more="items.length > 6 ? t('share.more', { n: items.length - 6 }) : undefined">
      <template #cell="{ column, value }"><span :class="{ mono: column.key === 'from' }">{{ value }}</span></template>
    </Table>
    <FieldRow :label="t('share.folderName')" :hint="t('share.folderNameTip')"><TextField id="cs-title" v-model="st.title" autocomplete="off" width="wide" /></FieldRow>
  </SubCard>
  <LinkOptions :f="f" name="sammlung" />
</template>
