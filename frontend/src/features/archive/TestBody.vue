<script setup lang="ts">
// Archiv testen: one row per archive with how far the reading got and what it found.
import { GlyphTile, Pill, ProgressBar, SubCard } from '@glimstone/vue';
import { G, glyphComponent, glyphOf } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { t } from '@/i18n';
import { plural } from '@/ui/kit';

export interface TestRow { k: Entry; path: string; size: number; members: number; done: number; state: 'waiting' | 'testing' | 'ok' | 'broken' | 'locked'; why: string }
defineProps<{ st: { rows: TestRow[]; sum: string } }>();
const ended = (r: TestRow) => ['ok', 'broken', 'locked'].includes(r.state);
</script>

<template>
  <SubCard><div id="at-sum" class="glim-eyebrow">{{ st.sum }}</div>
    <div id="at-list" class="at-list">
      <div v-for="(r, i) in st.rows" :key="r.path" class="at-row" :class="{ bad: r.state === 'broken' || r.state === 'locked', good: r.state === 'ok' }" :data-at="i">
        <GlyphTile :glyph="glyphComponent(glyphOf(r.k))" ground :quiet="!r.k.dir" /><span class="bk-name">{{ r.k.name }}</span><ProgressBar class="bar" :value="r.done" :label="r.k.name" :failed="r.state === 'broken' || r.state === 'locked'" />
        <Pill class="glim-num">{{ r.state === 'ok' ? t('archive.of', { done: r.members, total: r.members }) : ended(r) ? plural(r.members, 'ui.files') : Math.round(r.done * 100) + ' %' }}</Pill>
        <Pill class="at-state" :data-tip="r.why || undefined"><G v-if="r.state === 'broken' || r.state === 'locked'" n="IconWarning" /><G v-else-if="r.state === 'ok'" n="IconCheck" />{{ t(`archive.states.${r.state}`) }}</Pill>
      </div>
    </div>
  </SubCard>
</template>
