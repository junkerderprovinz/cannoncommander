<script setup lang="ts">
// Herunterladen of several entries or a folder: packed by the server while it sends, as zip or tar.gz.
import { computed, watch } from 'vue';
import { fmtSize } from '@/core/format';
import type { Entry } from '@/files/entries';
import { t } from '@/i18n';
import Frow from '@/ui/Frow.vue';
import { plural } from '@/ui/kit';
import Seg from '@/ui/Seg.vue';

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
    <Frow :label="t('share.format')" :tip="t('share.formatTip')"><Seg id="dl-fmt" v-model="st.format" :options="formats" /></Frow>
    <Frow :label="t('share.fileName')"><input id="dl-name" v-model="st.name" class="glim-input m"></Frow>
    <Frow :label="t('share.savesIn')" :tip="t('share.savesInTip')"><span class="pill">{{ t('share.browserFolder') }}</span></Frow>
  </div>
</template>
