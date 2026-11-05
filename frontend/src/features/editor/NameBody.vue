<script setup lang="ts">
// The name field of Neue Datei. Enter takes the name, and the part before the extension is selected to type over.
import { FieldRow, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';

const props = defineProps<{ value: string; close?: () => void }>();
const input = ref<InstanceType<typeof TextField> | null>(null);

onMounted(() => setTimeout(() => {
  const dot = props.value.lastIndexOf('.');
  input.value?.input?.setSelectionRange(0, dot > 0 ? dot : props.value.length);
}, 0));
const enter = (e: KeyboardEvent) => {
  if (e.key === 'Enter') (input.value?.input?.closest('.win')?.querySelector('.glim-modal-foot .glim-btn--accent') as HTMLElement | null)?.click();
};
</script>

<template>
  <div class="well glim-well"><FieldRow :label="t('editor.name')"><TextField id="nm" ref="input" :model-value="value" width="wide" @keydown="enter" /></FieldRow></div>
</template>
