<script setup lang="ts">
// The address, the file name it suggests, and the folder the file lands in.
import { t } from '@/i18n';
import PathField from '@/ui/PathField.vue';
import Frow from '@/ui/Frow.vue';
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
  <div class="well glim-well bk-form" @keydown.enter="pressPrimary">
    <Frow :label="t('fetch.address')" :tip="t('fetch.addressTip')"><input id="fu-url" v-model="st.url" class="glim-input grow glim-input--mono" placeholder="https://example.org/datei.iso" autocomplete="off" spellcheck="false" inputmode="url" @input="urlTyped"></Frow>
    <Frow :label="t('fetch.fileName')" :tip="t('fetch.fileNameTip')"><input id="fu-name" v-model="st.name" class="glim-input grow" autocomplete="off" spellcheck="false" @input="st.typed = !!st.name.trim()"></Frow>
    <Frow :label="t('fetch.target')" :tip="t('fetch.targetTip')"><PathField id="fu-dest" v-model="st.dest" /></Frow>
  </div>
</template>
