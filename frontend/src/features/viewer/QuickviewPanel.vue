<script setup lang="ts">
// With the preview bar on, the other panel shows what the cursor stands on, as Krusader's quick view does.
import { Card } from '@glimstone/vue';
import { computed } from 'vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import Preview from './Preview.vue';

defineProps<{ i: number }>();
const F = usePanels();
const k = computed(() => F.cursorEntry());
const folder = computed(() => k.value?.at || F.T().path);
</script>

<template>
  <Card class="panel glim-card" :title="t('viewer.preview')" :data-panel="i">
    <div id="quickview" class="quickview">
      <div v-if="!k || k.up" class="empty"><G n="IconEye" /><div>{{ t('viewer.pickFile') }}</div></div>
      <Preview v-else :key="folder + '/' + k.name" :n="k" :folder="folder" />
    </div>
  </Card>
</template>
