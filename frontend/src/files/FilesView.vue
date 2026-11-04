<script setup lang="ts">
// The file manager: the sidebar and one or two panels. The active panel's list keeps the focus, so keys go to it.
import { nextTick, onMounted, onUpdated, watch } from 'vue';
import { extend } from '@/core/extend';
import { prefs } from '@/core/prefs';
import { usePanels } from '@/stores/panels';
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
  <Sidebar /><div id="panels" class="panels"><Panel v-for="i in prefs.dual ? [0, 1] : [0]" :key="i" :i="i" /></div>
</template>
