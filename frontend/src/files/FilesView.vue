<script setup lang="ts">
// The file manager: the sidebar and one or two panels. The active panel's list keeps the focus, so keys go to it.
import { nextTick, onMounted, onUpdated, watch } from 'vue';
import { extend } from '@/core/extend';
import { prefs } from '@/core/prefs';
import { usePanels } from '@/stores/panels';
import QuickviewPanel from '@/features/viewer/QuickviewPanel.vue';
import Panel from './Panel.vue';
import Sidebar from './Sidebar.vue';

const F = usePanels();
const focusList = () => document.querySelector<HTMLElement>('.panel.active .list')?.focus({ preventScroll: true });

watch(() => prefs.dual, (dual) => { if (!dual) F.active = 0; });
watch(() => F.active, () => nextTick(focusList));
onMounted(focusList);
onUpdated(() => extend.onRender.forEach((fn) => fn()));
</script>

<template>
  <Sidebar /><div id="panels" class="panels"><template v-for="i in prefs.dual ? [0, 1] : [0]" :key="i"><QuickviewPanel v-if="prefs.quickview && prefs.dual && i !== F.active" :i="i" /><Panel v-else :i="i" /></template></div>
</template>
