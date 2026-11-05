<script setup lang="ts">
// Prüfsummen erstellen: the method, every file with its sum as it comes in, and where the list is saved.
import { pathField } from '@/ui/picker';
import { FieldRow, PathField, ProgressBar, Selector, SubCard, TextField } from '@glimstone/vue';
import { computed } from 'vue';
import type { Job } from '@/api/ops';
import type { Algo, ChecksumResult } from '@/api/tools';
import { fmtSize } from '@/core/format';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import CopyBtn from '@/ui/CopyBtn.vue';
import { plural, segItems } from '@/ui/kit';
import Tile from '@/ui/Tile.vue';

export interface SumRow { name: string; size: number; state: 'wait' | 'read' | 'done'; hash?: string }
export interface SumsState { algo: Algo; rows: SumRow[]; dir: string; name: string; touched: boolean; job: Job | null; result: ChecksumResult | null }
const props = defineProps<{ st: SumsState }>();
const jobs = useJobs();

const ALGOS: Algo[] = ['md5', 'sha1', 'sha256', 'sha512', 'b3'];
const algos = computed(() => ALGOS.map((a) => [a, t(`checksum.algos.${a}`)] as const));
const job = computed(() => (props.st.job ? jobs.find(props.st.job.id) : undefined));
const count = computed(() => {
  const files = job.value?.totalFiles || props.st.rows.length, bytes = job.value?.totalBytes || props.st.rows.reduce((a, r) => a + r.size, 0);
  return `${plural(files, 'ui.files')}, ${fmtSize(bytes) || '0 B'}`;
});
const width = (r: SumRow) => (r.state === 'done' ? 1 : r.state === 'read' ? 0.6 : 0);
</script>

<template>
  <SubCard class="bk-form"><FieldRow :label="t('checksum.algo')" :hint="t('checksum.algoTip')"><Selector id="c-algo" v-model="st.algo" :items="segItems(algos)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow></SubCard>
  <SubCard class="bk-fill"><div class="bk-tools"><span id="c-count" class="glim-eyebrow">{{ count }}</span></div>
    <div id="c-list" class="bk-list">
      <div v-for="(r, i) in st.rows" :id="`c-row-${i}`" :key="r.name" class="bk-sum">
        <Tile :k="{ name: r.name }" /><span class="bk-name" :title="r.name">{{ r.name }}</span>
        <span class="bk-hash mono"><span v-if="r.hash" :title="r.hash">{{ r.hash }}</span><ProgressBar v-else class="bar" :value="width(r)" :label="r.name" /></span>
        <CopyBtn v-if="r.hash" :text="r.hash" :tip="t('checksum.copySum')" /><span v-else></span>
      </div>
    </div>
  </SubCard>
  <SubCard :title="t('checksum.save')" class="bk-form">
    <FieldRow :label="t('checksum.folder')"><PathField v-model="st.dir" v-bind="pathField('c-dir')" /></FieldRow>
    <FieldRow :label="t('checksum.fileName')"><TextField id="c-name" v-model="st.name" spellcheck="false" autocomplete="off" width="grow" mono @input="st.touched = true" /></FieldRow>
  </SubCard>
</template>
