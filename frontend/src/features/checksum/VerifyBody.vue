<script setup lang="ts">
// Prüfsummen prüfen: a list from a file or pasted lines, and what each line found.
import { computed, nextTick, ref, watch } from 'vue';
import type { Algo } from '@/api/tools';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import BkPath from '@/ui/BkPath.vue';
import Frow from '@/ui/Frow.vue';
import { plural } from '@/ui/kit';
import Seg from '@/ui/Seg.vue';
import Tile from '@/ui/Tile.vue';

export interface VerifyRow { name: string; state: 'wait' | 'ok' | 'mismatch' | 'missing' | 'bad'; bad?: boolean; algo?: Algo; found?: boolean }
export interface VerifyState { mode: 'file' | 'paste'; file: string; text: string; dir: string; rows: VerifyRow[] | null; empty: 'pick' | 'noFile' | 'noSums'; seen: number }
const props = defineProps<{ st: VerifyState; check: () => void }>();

const modes = computed(() => [['file', t('checksum.fromFile')], ['paste', t('checksum.pasted')]] as const);
const fileField = ref<HTMLInputElement | null>(null), textField = ref<HTMLTextAreaElement | null>(null);
watch(() => props.st.mode, async (m) => { await nextTick(); (m === 'file' ? fileField.value : textField.value)?.focus(); });

const EMPTY = { pick: ['IconShieldOn', 'checksum.pick'], noFile: ['IconFile', 'checksum.noFile'], noSums: ['IconShieldOn', 'checksum.noSums'] } as const;
const count = computed(() => {
  const rows = props.st.rows;
  if (!rows) return t('checksum.result');
  const seen = rows.filter((r) => r.state !== 'wait'), c = (s: VerifyRow['state']) => seen.filter((r) => r.state === s).length;
  return [plural(rows.length, 'ui.files'), seen.length && t('checksum.okCount', { n: c('ok') }), c('mismatch') && t('checksum.offCount', { n: c('mismatch') }),
    c('missing') && t('checksum.missingCount', { n: c('missing') })].filter(Boolean).join(', ');
});
const width = (r: VerifyRow, i: number) => (r.state !== 'wait' || i < props.st.seen ? '100%' : i === props.st.seen ? '50%' : '0%');
</script>

<template>
  <div class="well bk-form">
    <Frow :label="t('checksum.sums')"><Seg id="v-mode" v-model="st.mode" :options="modes" /></Frow>
    <div class="bk-swap">
      <div class="bk-swap-a" :hidden="st.mode !== 'file'"><Frow :label="t('checksum.file')" :tip="t('checksum.fileTip')"><input id="v-file" ref="fileField" v-model="st.file" class="field grow mono" placeholder="/mnt/user/downloads/datei.sha256" spellcheck="false" autocomplete="off" @keydown.enter.prevent="check"></Frow></div>
      <div class="bk-swap-b" :hidden="st.mode !== 'paste'"><textarea id="v-text" ref="textField" v-model="st.text" class="field mono" rows="3" :placeholder="t('checksum.pastePlaceholder')" spellcheck="false"></textarea>
        <Frow :label="t('checksum.filesIn')"><BkPath id="v-dir" v-model="st.dir" /></Frow></div>
    </div>
  </div>
  <div class="well bk-fill"><div class="bk-tools"><span id="v-count" class="eyebrow">{{ count }}</span></div>
    <div id="v-list" class="bk-list">
      <div v-if="!st.rows" class="empty"><G :n="EMPTY[st.empty][0]" /><div>{{ t(EMPTY[st.empty][1]) }}</div></div>
      <template v-else><div v-for="(r, i) in st.rows" :id="`v-row-${i}`" :key="i" class="bk-sum">
        <Tile v-if="r.found !== false && !r.bad" :k="{ name: r.name }" /><Tile v-else g="IconFile" />
        <span class="bk-name mono">{{ r.name }}</span>
        <span class="bk-hash"><span v-if="r.state !== 'wait' && r.algo" class="muted">{{ t(`checksum.algos.${r.algo}`) }}</span><span v-else-if="!r.bad" class="bar"><i :style="{ width: width(r, i) }"></i></span></span>
        <span class="bk-res">
          <span v-if="r.state === 'ok'" class="bk-ok"><span class="bk-dot"></span>{{ t('checksum.ok') }}</span>
          <span v-else-if="r.state === 'mismatch'" class="pill fault">{{ t('checksum.mismatch') }}</span>
          <span v-else-if="r.state === 'missing'" class="pill warn">{{ t('checksum.missing') }}</span>
          <span v-else-if="r.state === 'bad'" class="pill warn">{{ t('checksum.bad') }}</span>
        </span>
      </div></template>
    </div>
  </div>
</template>
