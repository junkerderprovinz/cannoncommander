<script setup lang="ts">
// The name field of Neue Datei. Enter takes the name, and the part before the extension is selected to type over.
import { FieldRow, SubCard, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { stemLength } from '@/files/paths';
import { t } from '@/i18n';

const props = defineProps<{ value: string; close?: () => void }>();
const input = ref<InstanceType<typeof TextField> | null>(null);

onMounted(() => setTimeout(() => input.value?.input?.setSelectionRange(0, stemLength(props.value, false)), 0));
const enter = (e: KeyboardEvent) => {
  if (e.key === 'Enter') (input.value?.input?.closest('.win')?.querySelector('.glim-modal-foot .glim-btn--accent') as HTMLElement | null)?.click();
};
</script>

<template>
  <SubCard><FieldRow :label="t('editor.name')"><TextField id="nm" ref="input" :model-value="value" width="wide" @keydown="enter" /></FieldRow></SubCard>
</template>
