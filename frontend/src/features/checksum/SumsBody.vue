<script setup lang="ts">
// Prüfsummen erstellen: the method, every file with its sum as it comes in, and where the list is saved.
import { computed } from 'vue';
import type { Job } from '@/api/ops';
import type { Algo, ChecksumResult } from '@/api/tools';
import { fmtSize } from '@/core/format';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import BkPath from '@/ui/BkPath.vue';
import CopyBtn from '@/ui/CopyBtn.vue';
import Frow from '@/ui/Frow.vue';
import { plural } from '@/ui/kit';
import Seg from '@/ui/Seg.vue';
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
const width = (r: SumRow) => (r.state === 'done' ? '100%' : r.state === 'read' ? '60%' : '0%');
</script>

<template>
  <div class="well bk-form"><Frow :label="t('checksum.algo')" :tip="t('checksum.algoTip')"><Seg id="c-algo" v-model="st.algo" :options="algos" /></Frow></div>
  <div class="well bk-fill"><div class="bk-tools"><span id="c-count" class="eyebrow">{{ count }}</span></div>
    <div id="c-list" class="bk-list">
      <div v-for="(r, i) in st.rows" :id="`c-row-${i}`" :key="r.name" class="bk-sum">
        <Tile :k="{ name: r.name }" /><span class="bk-name" :title="r.name">{{ r.name }}</span>
        <span class="bk-hash mono"><span v-if="r.hash" :title="r.hash">{{ r.hash }}</span><span v-else class="bar"><i :style="{ width: width(r) }"></i></span></span>
        <CopyBtn v-if="r.hash" :text="r.hash" :tip="t('checksum.copySum')" /><span v-else></span>
      </div>
    </div>
  </div>
  <div class="well bk-form"><div class="eyebrow">{{ t('checksum.save') }}</div>
    <Frow :label="t('checksum.folder')"><BkPath id="c-dir" v-model="st.dir" /></Frow>
    <Frow :label="t('checksum.fileName')"><input id="c-name" v-model="st.name" class="field grow mono" spellcheck="false" autocomplete="off" @input="st.touched = true"></Frow>
  </div>
</template>
