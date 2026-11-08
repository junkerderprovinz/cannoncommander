<script setup lang="ts">
// The way from the app's Eigenschaften to the system's own dialog, with security, details and previous versions on
// Windows, or the dialog of the file manager a Linux desktop brings.
import { Button, FieldRow, SubCard } from '@glimstone/vue';
import { host } from '@/core/host';
import { t } from '@/i18n';
import { systemProperties } from './shell';

const props = defineProps<{ path: string }>();
const windows = host.os === 'windows';
</script>

<template>
  <SubCard class="bk-form sm-props">
    <FieldRow :label="t(windows ? 'shellmenu.propsWindows' : 'shellmenu.propsSystem')" :hint="t(windows ? 'shellmenu.propsWindowsTip' : 'shellmenu.propsSystemTip')">
      <Button id="sm-props" :label="t('shellmenu.propsOpen')" label-key="IconExternalLink" @click="systemProperties([props.path])" />
    </FieldRow>
  </SubCard>
</template>
