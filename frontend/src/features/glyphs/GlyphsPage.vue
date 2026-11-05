<script setup lang="ts">
// Every glyph GlimStone has for folders, places and files, with what CannonCommander uses it for, and the kinds still
// without one.
import { Card } from '@glimstone/vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import data from './sets.json';

const sets = data.sets as [string, string[]][];
const missing = data.missing as [string, string][];
</script>

<template>
  <div class="page-inner gl">
    <Card v-for="[id, names] in sets" :key="id" :title="t(`glyphs.${id}`)">
      <div class="gl-grid">
        <div v-for="n in names" :key="n" class="gl-tile"><span class="gl-g"><G :n="n" /></span><div><b>{{ t(`glyphs.g.${n}.label`) }}</b><div class="gl-use">{{ t(`glyphs.g.${n}.use`) }}</div><div class="gl-name mono">{{ n }}</div></div></div>
      </div>
    </Card>
    <Card :title="t('glyphs.missing')">
      <div class="gl-grid">
        <div v-for="[id, now] in missing" :key="id" class="gl-tile"><span class="gl-g muted-g"><G :n="now" /></span><div><b>{{ t(`glyphs.m.${id}`) }}</b><div class="gl-use">{{ t('glyphs.uses', { g: now }) }}</div></div></div>
      </div>
    </Card>
  </div>
</template>

<style>
.gl-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(17rem, 1fr)); gap: .5rem 1.25rem; }
.gl-tile { display: flex; gap: .85rem; align-items: flex-start; padding: .55rem .25rem; }
.gl-g svg { width: 2rem; height: 2rem; color: var(--accent-glyph); }
.gl-g.muted-g svg { color: var(--carbon-text-muted); }
.gl-use { font-size: 13px; color: var(--carbon-text-sub); }
.gl-name { font-size: 12px; color: var(--carbon-text-muted); margin-top: .15rem; }
</style>
