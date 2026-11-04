<script setup lang="ts">
// The phone's bar at the bottom: files and settings are places, search and jobs open their windows.
import { run } from '@/core/actions';
import { G } from '@/core/glyphs';
import { go, view } from '@/core/views';
import { t } from '@/i18n';

const TABS = [['files', 'IconFiles'], ['search', 'IconSearch'], ['jobs', 'IconQueued'], ['settings', 'IconGear']] as const;
const pick = (id: string) => (id === 'files' || id === 'settings' ? go(id) : run(id));
</script>

<template>
  <nav class="bottombar glim-card">
    <button v-for="[id, glyph] in TABS" :key="id" :data-bottom="id" :aria-current="view.id === id ? 'page' : undefined" @click="pick(id)"><G :n="glyph" /><span>{{ t(`bottom.${id}`) }}</span></button>
  </nav>
</template>
