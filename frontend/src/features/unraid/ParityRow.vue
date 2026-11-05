<script setup lang="ts">
// The parity run in the sidebar while it lasts. Big jobs wait for it, so a click shows them.
import { SidebarRow } from '@glimstone/vue';
import { computed } from 'vue';
import { run } from '@/core/actions';
import { glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { useUnraid } from './state';

const U = useUnraid();
const p = computed(() => U.st?.parity);
</script>

<template>
  <SidebarRow v-if="p?.running" :label="t(`unraid.parity.${p.kind || 'check'}`)" :glyph="glyphComponent('IconShieldOn')" data-parity :data-tip="t(p.paused ? 'unraid.parityPausedTip' : 'unraid.parityTip')" @click="run('jobs')"><template #after><span class="meta glim-num">{{ t('unraid.pct', { pct: Math.floor(p.percent) }) }}</span></template></SidebarRow>
</template>
