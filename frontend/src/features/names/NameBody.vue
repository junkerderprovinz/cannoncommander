<script setup lang="ts">
// One name to type. The part before the extension starts selected, and Enter takes it.
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';
import Frow from '@/ui/Frow.vue';

const props = defineProps<{ st: { name: string }; hint?: string; field?: string }>();
const input = ref<HTMLInputElement | null>(null);
onMounted(() => setTimeout(() => {
  const dot = props.st.name.lastIndexOf('.');
  input.value?.setSelectionRange(0, dot > 0 ? dot : props.st.name.length);
}, 0));
const enter = (e: KeyboardEvent) => (e.target as Element).closest('.win')?.querySelector<HTMLElement>('.glim-modal-foot .glim-btn--accent')?.click();
</script>

<template>
  <div class="well glim-well"><Frow :label="t('names.name')" :tip="hint"><input :id="field || 'nm'" ref="input" v-model="st.name" class="glim-input w" autocomplete="off" spellcheck="false" @keydown.enter="enter"></Frow></div>
</template>
