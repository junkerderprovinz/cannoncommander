<script setup lang="ts">
// Herunterladen of several entries or a folder: packed by the server while it sends, as zip or tar.gz.
import { FieldRow, Pill, Selector, TextField } from '@glimstone/vue';
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
  <div class="well glim-well"><div class="eyebrow">{{ plural(st.items.length, 'ui.objects') }}, {{ fmtSize(total) }}</div>
    <table class="tbl">
      <tr v-for="k in st.items.slice(0, 6)" :key="k.name"><td>{{ k.name }}</td><td class="glim-num r">{{ fmtSize(k.size) || '0 B' }}</td></tr>
      <tr v-if="st.items.length > 6"><td colspan="2">{{ t('share.more', { n: st.items.length - 6 }) }}</td></tr>
    </table>
  </div>
  <div class="well glim-well"><div class="eyebrow">{{ t('share.archive') }}</div>
    <FieldRow :label="t('share.format')" :hint="t('share.formatTip')"><Selector id="dl-fmt" v-model="st.format" :items="segItems(formats)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="t('share.fileName')"><TextField id="dl-name" v-model="st.name" width="medium" /></FieldRow>
    <FieldRow :label="t('share.savesIn')" :hint="t('share.savesInTip')"><Pill>{{ t('share.browserFolder') }}</Pill></FieldRow>
  </div>
</template>
