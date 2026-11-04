<script setup lang="ts">
// The bar at the bottom while rows are ticked by touch, with what a phone's gallery offers for a selection.
import { computed } from 'vue';
import { run } from '@/core/actions';
import { G } from '@/core/glyphs';
import { view } from '@/core/views';
import { ticking } from '@/files/touch';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { stop } from './state';

const F = usePanels();
const ACTS = [['copy', 'IconCopy'], ['move', 'IconMove'], ['share', 'IconLink'], ['delete', 'IconTrash']] as const;
const n = computed(() => F.T().marked.size);
</script>

<template>
  <div v-if="view.id === 'files' && ticking(F.T().path)" class="touchbar glim-card" role="toolbar" :aria-label="t('touch.bar')">
    <span class="tb-count glim-num">{{ n ? t('touch.marked', { n }) : t('touch.tap') }}</span>
    <button v-for="[id, g] in ACTS" :key="id" :data-run="id" :disabled="!n" @click="run(id)"><G :n="g" /><span>{{ t(`touch.${id}`) }}</span></button>
    <button data-tsel-done class="tb-done" @click="stop"><G n="IconCheck" /><span>{{ t('touch.done') }}</span></button>
  </div>
</template>
