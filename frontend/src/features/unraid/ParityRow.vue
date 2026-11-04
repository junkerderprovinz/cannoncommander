<script setup lang="ts">
// The parity run in the sidebar while it lasts. Big jobs wait for it, so a click shows them.
import { computed } from 'vue';
import { run } from '@/core/actions';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { useUnraid } from './state';

const U = useUnraid();
const p = computed(() => U.st?.parity);
</script>

<template>
  <div v-if="p?.running" class="nav-row" data-parity :data-tip="t(p.paused ? 'unraid.parityPausedTip' : 'unraid.parityTip')" @click="run('jobs')">
    <G n="IconShieldOn" /><span>{{ t(`unraid.parity.${p.kind || 'check'}`) }}</span><span class="meta glim-num">{{ t('unraid.pct', { pct: Math.floor(p.percent) }) }}</span>
  </div>
</template>
