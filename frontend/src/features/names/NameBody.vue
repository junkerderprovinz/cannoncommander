<script setup lang="ts">
// One name to type, selected as a whole, or up to the extension for a new file. Enter takes it.
import { FieldRow, SubCard, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';

const props = defineProps<{ st: { name: string }; hint?: string; field?: string; stem?: boolean }>();
const input = ref<InstanceType<typeof TextField> | null>(null);
onMounted(() => setTimeout(() => {
  const dot = props.stem ? props.st.name.lastIndexOf('.') : -1;
  input.value?.input?.setSelectionRange(0, dot > 0 ? dot : props.st.name.length);
}, 0));
const enter = (e: KeyboardEvent) => (e.target as Element).closest('.win')?.querySelector<HTMLElement>('.glim-modal-foot .glim-btn--accent')?.click();
</script>

<template>
  <SubCard><FieldRow :label="t('names.name')" :hint="hint"><TextField :id="field || 'nm'" ref="input" v-model="st.name" autocomplete="off" spellcheck="false" width="wide" @keydown.enter="enter" /></FieldRow></SubCard>
</template>
