<script setup lang="ts">
// Gehe zu Zeile (Ctrl+G): a line, or a line and a column as 12:5. Enter jumps.
import { FieldRow, SubCard, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';

defineProps<{ value: string; lines: number; close?: () => void }>();
const input = ref<InstanceType<typeof TextField> | null>(null);

onMounted(() => setTimeout(() => input.value?.input?.select(), 0));
const enter = (e: KeyboardEvent) => {
  if (e.key === 'Enter') (input.value?.input?.closest('.win')?.querySelector('.glim-modal-foot .glim-btn--accent') as HTMLElement | null)?.click();
};
</script>

<template>
  <SubCard>
    <FieldRow :label="t('editor.gotoField')" :hint="t('editor.gotoHint', { n: lines })">
      <TextField id="ce-goto" ref="input" :model-value="value" inputmode="numeric" width="short" autocomplete="off" @keydown="enter" />
    </FieldRow>
  </SubCard>
</template>
