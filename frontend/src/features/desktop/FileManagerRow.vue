<script setup lang="ts">
// The switch for the computer's file manager, in the settings beside the start folder. It shows only where the app can
// take that place.
import { FieldRow, Toggle } from '@glimstone/vue';
import { onMounted } from 'vue';
import { host } from '@/core/host';
import { t } from '@/i18n';
import { fileManager, loadFileManager, setFileManager } from './filemanager';

onMounted(loadFileManager);
</script>

<template>
  <FieldRow v-if="fileManager.offered" :label="t('desktop.fmLabel')" :hint="t(host.os === 'windows' ? 'desktop.fmTipWindows' : 'desktop.fmTipLinux')">
    <Toggle id="d-filemanager" :model-value="fileManager.on" @update:model-value="setFileManager" />
  </FieldRow>
</template>
