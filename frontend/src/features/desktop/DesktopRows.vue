<script setup lang="ts">
// The desktop app's own settings beside the start folder: the program F4 opens text files in, and whether the app takes
// the computer's file manager's place, which shows only where it can. How it handles lives with the keys.
import { FieldRow, Toggle } from '@glimstone/vue';
import { onMounted } from 'vue';
import { host } from '@/core/host';
import ProgramRow from '@/features/editor/ProgramRow.vue';
import { t } from '@/i18n';
import { fileManager, loadFileManager, setFileManager } from './filemanager';

onMounted(loadFileManager);
</script>

<template>
  <ProgramRow />
  <FieldRow v-if="fileManager.offered" :label="t('desktop.fmLabel')" :hint="t(host.os === 'windows' ? 'desktop.fmTipWindows' : 'desktop.fmTipLinux')">
    <Toggle id="d-filemanager" :model-value="fileManager.on" @update:model-value="setFileManager" />
  </FieldRow>
</template>
