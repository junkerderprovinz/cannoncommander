<script setup lang="ts">
// The settings tile: the layout to start from, which is how the app handles, and every key, changed at once.
import { segItems } from '@/ui/kit';
import { Button, Card, FieldRow, Kbd, Selector, TextField, Toggle } from '@glimstone/vue';
import { computed, ref } from 'vue';
import { host } from '@/core/host';
import { keymap } from '@/core/keymap';
import { prefs, setPref } from '@/core/prefs';
import { zoomMod } from '@/files/zoom';
import { t } from '@/i18n';
import type { KeyMap, Preset } from './keys';
import KeysList from './KeysList.vue';
import { commit, hasOwnKeys, layout, layoutItems, layoutKeys, useLayout } from './state';

const query = ref('');
const base = computed(() => layoutKeys(layout()));
const FIXED = ['tab', 'enter', 'insert', 'space', 'f10', 'esc', 'zoom'];
const change = (map: KeyMap) => commit(map, layout());
</script>

<template>
  <Card :title="t('keys.preset')">
    <FieldRow :label="t('keys.style')" :hint="t(host.desktop ? 'keys.styleTipDesktop' : 'keys.styleTip')">
      <Selector id="kt-layout" :model-value="layout()" :items="segItems(layoutItems())" size="sm" activation="manual" inline button-height pairs-on-phone @update:model-value="useLayout($event as Preset)" />
    </FieldRow>
    <FieldRow v-if="host.desktop" :label="t('keys.clickSelect')" :hint="t('keys.clickSelectTip')">
      <Toggle id="d-clickselect" :model-value="!!prefs.clickSelect" @update:model-value="setPref('clickSelect', $event)" />
    </FieldRow>
    <FieldRow :label="t('keys.own')" :hint="t('keys.resetTip')">
      <Button id="kt-reset" :label="t('keys.reset')" label-key="IconRefresh" :disabled="!hasOwnKeys()" @click="change({ ...base })" />
    </FieldRow>
    <FieldRow :label="t('keys.search')">
      <TextField id="kt-q" v-model="query" :placeholder="t('keys.searchPh')" autocomplete="off" width="medium" />
    </FieldRow>
  </Card>
  <KeysList :map="{ ...keymap }" :base="base" :query="query" cards @update:map="change" />
  <Card :title="t('keys.fixedHead')">
    <div class="kbd-grid"><div v-for="k in FIXED" :key="k"><span>{{ t(`keys.fixedKeys.${k}.label`) }}</span><Kbd>{{ t(`keys.fixedKeys.${k}.key`, { mod: zoomMod() }) }}</Kbd></div></div>
  </Card>
</template>
