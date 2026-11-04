<script setup lang="ts">
// With the preview bar on, the other panel shows what the cursor stands on, as Krusader's quick view does: its name
// and its size.
import { Pill } from '@glimstone/vue';
import { computed } from 'vue';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';

defineProps<{ i: number }>();
const F = usePanels();
const k = computed(() => F.cursorEntry());
</script>

<template>
  <section class="panel glim-card" :data-panel="i">
    <span class="notch">{{ t('files.preview') }}</span>
    <div id="quickview" class="quickview">
      <div v-if="!k || k.up" class="empty"><G n="IconEye" /><div>{{ t('files.previewPick') }}</div></div>
      <div v-else class="well"><div class="frow"><span class="lab">{{ k.name }}</span><Pill class="pill">{{ fmtSize(k.size) }}</Pill></div></div>
    </div>
  </section>
</template>
