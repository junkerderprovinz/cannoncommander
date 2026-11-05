<script setup lang="ts">
// One name to type. The part before the extension starts selected, and Enter takes it.
import { FieldRow, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';

const props = defineProps<{ st: { name: string }; hint?: string; field?: string }>();
const input = ref<InstanceType<typeof TextField> | null>(null);
onMounted(() => setTimeout(() => {
  const dot = props.st.name.lastIndexOf('.');
  input.value?.input?.setSelectionRange(0, dot > 0 ? dot : props.st.name.length);
}, 0));
const enter = (e: KeyboardEvent) => (e.target as Element).closest('.win')?.querySelector<HTMLElement>('.glim-modal-foot .glim-btn--accent')?.click();
</script>

<template>
  <div class="well glim-well"><FieldRow :label="t('names.name')" :hint="hint"><TextField :id="field || 'nm'" ref="input" v-model="st.name" autocomplete="off" spellcheck="false" width="wide" @keydown.enter="enter" /></FieldRow></div>
</template>
