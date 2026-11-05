<script setup lang="ts">
// Archiv testen: one row per archive with how far the reading got and what it found.
import { G } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { t } from '@/i18n';
import { plural } from '@/ui/kit';
import Tile from '@/ui/Tile.vue';

export interface TestRow { k: Entry; path: string; size: number; members: number; done: number; state: 'waiting' | 'testing' | 'ok' | 'broken' | 'locked'; why: string }
defineProps<{ st: { rows: TestRow[]; sum: string } }>();
const ended = (r: TestRow) => ['ok', 'broken', 'locked'].includes(r.state);
</script>

<template>
  <div class="well glim-well"><div id="at-sum" class="eyebrow">{{ st.sum }}</div>
    <div id="at-list" class="at-list">
      <div v-for="(r, i) in st.rows" :key="r.path" class="at-row" :class="{ bad: r.state === 'broken' || r.state === 'locked', good: r.state === 'ok' }" :data-at="i">
        <Tile :k="r.k" /><span class="bk-name">{{ r.k.name }}</span><span class="bar"><i :style="{ width: (r.done * 100).toFixed(1) + '%' }"></i></span>
        <span class="pill glim-num">{{ r.state === 'ok' ? t('archive.of', { done: r.members, total: r.members }) : ended(r) ? plural(r.members, 'ui.files') : Math.round(r.done * 100) + ' %' }}</span>
        <span class="pill at-state" :data-tip="r.why || undefined"><G v-if="r.state === 'broken' || r.state === 'locked'" n="IconWarning" /><G v-else-if="r.state === 'ok'" n="IconCheck" />{{ t(`archive.states.${r.state}`) }}</span>
      </div>
    </div>
  </div>
</template>
