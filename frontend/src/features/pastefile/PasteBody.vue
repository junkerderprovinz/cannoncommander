<script setup lang="ts">
// What came from the clipboard, and the name it gets. The part before the extension starts selected.
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';
import Frow from '@/ui/Frow.vue';

const props = defineProps<{ st: { name: string }; url?: string; text?: string; folder: string }>();
const input = ref<HTMLInputElement | null>(null);
onMounted(() => setTimeout(() => {
  const dot = props.st.name.lastIndexOf('.');
  input.value?.setSelectionRange(0, dot > 0 ? dot : props.st.name.length);
}, 0));
const enter = (e: KeyboardEvent) => (e.target as Element).closest('.win')?.querySelector<HTMLElement>('.win-foot .btn.primary')?.click();
</script>

<template>
  <div class="well pz-prev"><img v-if="url" class="pz-img" :src="url" alt=""><pre v-else class="pz-text">{{ text?.slice(0, 600) }}</pre></div>
  <div class="well"><Frow :label="t('pastefile.name')" :tip="t('pastefile.where', { path: folder })"><input id="pz-name" ref="input" v-model="st.name" class="field w" autocomplete="off" @keydown.enter="enter"></Frow></div>
</template>
