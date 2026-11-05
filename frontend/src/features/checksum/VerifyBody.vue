<script setup lang="ts">
// Prüfsummen prüfen: a list from a file or pasted lines, and what each line found.
import { pathField } from '@/ui/picker';
import { EmptyState, FieldRow, GlyphTile, PathField, Pill, ProgressBar, Selector, SubCard, TextArea, TextField } from '@glimstone/vue';
import { computed, nextTick, ref, watch } from 'vue';
import type { Algo } from '@/api/tools';
import { glyphComponent, glyphOf } from '@/core/glyphs';
import { t } from '@/i18n';
import { plural, segItems } from '@/ui/kit';

export interface VerifyRow { name: string; state: 'wait' | 'ok' | 'mismatch' | 'missing' | 'bad'; bad?: boolean; algo?: Algo; found?: boolean }
export interface VerifyState { mode: 'file' | 'paste'; file: string; text: string; dir: string; rows: VerifyRow[] | null; empty: 'pick' | 'noFile' | 'noSums'; seen: number }
const props = defineProps<{ st: VerifyState; check: () => void }>();

const modes = computed(() => [['file', t('checksum.fromFile')], ['paste', t('checksum.pasted')]] as const);
const fileField = ref<InstanceType<typeof TextField> | null>(null), textField = ref<InstanceType<typeof TextArea> | null>(null);
watch(() => props.st.mode, async (m) => { await nextTick(); (m === 'file' ? fileField.value?.input : textField.value?.input)?.focus(); });

const EMPTY = { pick: ['IconShieldOn', 'checksum.pick'], noFile: ['IconFile', 'checksum.noFile'], noSums: ['IconShieldOn', 'checksum.noSums'] } as const;
const count = computed(() => {
  const rows = props.st.rows;
  if (!rows) return t('checksum.result');
  const seen = rows.filter((r) => r.state !== 'wait'), c = (s: VerifyRow['state']) => seen.filter((r) => r.state === s).length;
  return [plural(rows.length, 'ui.files'), seen.length && t('checksum.okCount', { n: c('ok') }), c('mismatch') && t('checksum.offCount', { n: c('mismatch') }),
    c('missing') && t('checksum.missingCount', { n: c('missing') })].filter(Boolean).join(', ');
});
const width = (r: VerifyRow, i: number) => (r.state !== 'wait' || i < props.st.seen ? 1 : i === props.st.seen ? 0.5 : 0);
</script>

<template>
  <SubCard class="bk-form">
    <FieldRow :label="t('checksum.sums')"><Selector id="v-mode" v-model="st.mode" :items="segItems(modes)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <div class="bk-swap">
      <div class="bk-swap-a" :hidden="st.mode !== 'file'"><FieldRow :label="t('checksum.file')" :hint="t('checksum.fileTip')"><TextField id="v-file" ref="fileField" v-model="st.file" placeholder="/mnt/user/downloads/datei.sha256" spellcheck="false" autocomplete="off" width="grow" mono @keydown.enter.prevent="check" /></FieldRow></div>
      <div class="bk-swap-b" :hidden="st.mode !== 'paste'"><TextArea id="v-text" ref="textField" v-model="st.text" rows="3" :placeholder="t('checksum.pastePlaceholder')" spellcheck="false" mono />
        <FieldRow :label="t('checksum.filesIn')"><PathField v-model="st.dir" v-bind="pathField('v-dir')" /></FieldRow></div>
    </div>
  </SubCard>
  <SubCard class="bk-fill"><div class="bk-tools"><span id="v-count" class="glim-eyebrow">{{ count }}</span></div>
    <div id="v-list" class="bk-list">
      <EmptyState v-if="!st.rows" nested :glyph="glyphComponent(EMPTY[st.empty][0])" :title="t(EMPTY[st.empty][1])" />
      <template v-else><div v-for="(r, i) in st.rows" :id="`v-row-${i}`" :key="i" class="bk-sum">
        <GlyphTile v-if="r.found !== false && !r.bad" :glyph="glyphComponent(glyphOf({ name: r.name }))" ground quiet /><GlyphTile v-else :glyph="glyphComponent('IconFile')" ground quiet />
        <span class="bk-name mono">{{ r.name }}</span>
        <span class="bk-hash"><span v-if="r.state !== 'wait' && r.algo" class="muted">{{ t(`checksum.algos.${r.algo}`) }}</span><ProgressBar v-else-if="!r.bad" class="bar" :value="width(r, i)" :label="r.name" /></span>
        <span class="bk-res">
          <span v-if="r.state === 'ok'" class="bk-ok"><span class="bk-dot"></span>{{ t('checksum.ok') }}</span>
          <Pill v-else-if="r.state === 'mismatch'" tone="fail">{{ t('checksum.mismatch') }}</Pill>
          <Pill v-else-if="r.state === 'missing'" tone="warn">{{ t('checksum.missing') }}</Pill>
          <Pill v-else-if="r.state === 'bad'" tone="warn">{{ t('checksum.bad') }}</Pill>
        </span>
      </div></template>
    </div>
  </SubCard>
</template>
