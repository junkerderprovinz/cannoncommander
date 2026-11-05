<script setup lang="ts">
// The address, the file name it suggests, and the folder the file lands in.
import { pathField } from '@/ui/picker';
import { FieldRow, FieldRowGroup, PathField, SubCard, TextField } from '@glimstone/vue';
import { t } from '@/i18n';
import { pressPrimary } from '@/ui/kit';
import { parseUrl } from './url';

export interface FetchState { url: string; name: string; typed: boolean; dest: string }
const props = defineProps<{ st: FetchState }>();

function nameFrom(u: URL) {
  const last = u.pathname.split('/').filter(Boolean).pop();
  try {
    return decodeURIComponent(last || '') || u.hostname;
  } catch {
    return last || u.hostname;
  }
}
function urlTyped() {
  const u = parseUrl(props.st.url);
  if (u && !props.st.typed) props.st.name = nameFrom(u);
}
</script>

<template>
  <SubCard class="bk-form" @keydown.enter="pressPrimary">
    <FieldRowGroup>
      <FieldRow :label="t('fetch.address')" :hint="t('fetch.addressTip')"><TextField id="fu-url" v-model="st.url" placeholder="https://example.org/datei.iso" autocomplete="off" spellcheck="false" inputmode="url" width="grow" mono @input="urlTyped" /></FieldRow>
      <FieldRow :label="t('fetch.fileName')" :hint="t('fetch.fileNameTip')"><TextField id="fu-name" v-model="st.name" autocomplete="off" spellcheck="false" width="grow" @input="st.typed = !!st.name.trim()" /></FieldRow>
      <FieldRow :label="t('fetch.target')" :hint="t('fetch.targetTip')"><PathField v-model="st.dest" v-bind="pathField('fu-dest')" /></FieldRow>
    </FieldRowGroup>
  </SubCard>
</template>
