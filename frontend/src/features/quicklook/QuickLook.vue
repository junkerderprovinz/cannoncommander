<script setup lang="ts">
// Quick look: the file under the cursor large, without leaving the list, as on a Mac.
import { computed } from 'vue';
import { prefs } from '@/core/prefs';
import Preview from '@/features/viewer/Preview.vue';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { look } from './state';

const F = usePanels();
const k = computed(() => (look.open ? F.cursorEntry() : undefined));
const folder = computed(() => k.value?.at || F.T().path);
const count = computed(() => t('quicklook.of', { i: F.T().cursor + 1, n: F.entries().length }));
const beside = (e: MouseEvent) => { if (e.target === e.currentTarget) look.open = false; };
</script>

<template>
  <div v-if="look.open && k" class="ql-scrim" @mousedown="beside">
    <div class="ql glim-card" :class="{ 'with-info': !!prefs.quickInfo }" role="dialog" :aria-label="t('quicklook.title')">
      <span class="notch"><span class="ql-name">{{ k.name }}</span><span class="ql-count glim-num">{{ count }}</span></span>
      <div class="ql-body"><Preview :key="folder + '/' + k.name" :n="k" :folder="folder" autoplay /></div>
    </div>
  </div>
</template>
