<script setup lang="ts">
// The desktop app's own settings beside the start folder: how it handles, and whether it takes the computer's file
// manager's place, which shows only where it can.
import { FieldRow, Selector, Toggle } from '@glimstone/vue';
import { onMounted } from 'vue';
import { host } from '@/core/host';
import { prefs, setPref } from '@/core/prefs';
import { t } from '@/i18n';
import { segItems } from '@/ui/kit';
import { fileManager, loadFileManager, setFileManager } from './filemanager';
import { type Style, useStyle } from './style';

const STYLES: [Style, string][] = [['commander', t('desktop.commanderName')], ['explorer', t('desktop.explorerName')]];

onMounted(loadFileManager);
</script>

<template>
  <FieldRow :label="t('desktop.style')" :hint="t('desktop.styleTip')">
    <Selector :model-value="prefs.desktopStyle || 'commander'" :items="segItems(STYLES)" size="sm" activation="manual" inline button-height pairs-on-phone @update:model-value="useStyle" />
  </FieldRow>
  <FieldRow :label="t('desktop.clickSelect')" :hint="t('desktop.clickSelectTip')">
    <Toggle id="d-clickselect" :model-value="!!prefs.clickSelect" @update:model-value="setPref('clickSelect', $event)" />
  </FieldRow>
  <FieldRow v-if="fileManager.offered" :label="t('desktop.fmLabel')" :hint="t(host.os === 'windows' ? 'desktop.fmTipWindows' : 'desktop.fmTipLinux')">
    <Toggle id="d-filemanager" :model-value="fileManager.on" @update:model-value="setFileManager" />
  </FieldRow>
</template>
