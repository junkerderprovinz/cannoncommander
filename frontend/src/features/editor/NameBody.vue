<script setup lang="ts">
// The name field of Neue Datei. Enter takes the name, and the part before the extension is selected to type over.
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';

const props = defineProps<{ value: string; close?: () => void }>();
const input = ref<HTMLInputElement | null>(null);

onMounted(() => setTimeout(() => {
  const dot = props.value.lastIndexOf('.');
  input.value?.setSelectionRange(0, dot > 0 ? dot : props.value.length);
}, 0));
const enter = (e: KeyboardEvent) => {
  if (e.key === 'Enter') (input.value?.closest('.win')?.querySelector('.win-foot .btn.primary') as HTMLElement | null)?.click();
};
</script>

<template>
  <div class="well"><div class="frow"><span class="lab">{{ t('editor.name') }}</span><input id="nm" ref="input" class="field w" :value="value" @keydown="enter"></div></div>
</template>
