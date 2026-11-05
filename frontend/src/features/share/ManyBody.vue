<script setup lang="ts">
// Sammelfreigabe: entries from several folders under one link, which the other side sees as one folder.
import { FieldRow, TextField } from '@glimstone/vue';
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
  <div class="well glim-well"><div class="eyebrow">{{ t('share.fromFolders', { objects: plural(items.length, 'ui.objects'), folders: plural(folders, 'share.folders') }) }}, {{ size }}</div>
    <table class="tbl">
      <tr v-for="x in items.slice(0, 6)" :key="x.path"><td>{{ x.name }}</td><td class="muted mono">{{ parent(x.path) }}</td></tr>
      <tr v-if="items.length > 6"><td colspan="2">{{ t('share.more', { n: items.length - 6 }) }}</td></tr>
    </table>
    <FieldRow :label="t('share.folderName')" :hint="t('share.folderNameTip')"><TextField id="cs-title" v-model="st.title" autocomplete="off" width="wide" /></FieldRow>
  </div>
  <LinkOptions :f="f" name="sammlung" />
</template>
