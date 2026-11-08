<script setup lang="ts">
// F1: the keys to change in one go and take over at the end, beside the settings tile that changes them at once.
import { segItems } from '@/ui/kit';
import { FieldRow, Kbd, Selector, SubCard, TextField } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { host } from '@/core/host';
import { keymap } from '@/core/keymap';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { zoomMod } from '@/files/zoom';
import { t } from '@/i18n';
import { stepFoot } from '@/features/config/shared';
import { type KeyMap, ownKeys, type Preset, withOwn } from './keys';
import KeysList from './KeysList.vue';
import { layout, layoutItems, layoutKeys, shippedKeys } from './state';

const props = defineProps<{ win: WinEntry; apply: (map: KeyMap, which: Preset) => void }>();
const map = ref<KeyMap>({ ...keymap });
const which = ref(layout());
const base = computed(() => layoutKeys(which.value));
const query = ref('');
const FIXED = ['tab', 'enter', 'insert', 'space', 'f10', 'esc', 'zoom'];

// Own keys stay on top of the layout picked.
function pickLayout(v: Preset) {
  map.value = withOwn(layoutKeys(v), ownKeys(base.value, map.value));
  which.value = v;
}

onMounted(() => {
  stepFoot(props.win, [
    { start: t('keys.hint') },
    { label: t('keys.resetAll'), g: 'IconRefresh', run: () => { map.value = { ...shippedKeys() }; which.value = 'cc'; return false; } },
    cancelBtn(),
    { label: t('labels.take'), g: 'IconCheck', primary: true, run: () => props.apply(map.value, which.value) },
  ]);
});
</script>

<template>
  <SubCard>
    <FieldRow :label="t('keys.style')" :hint="t(host.desktop ? 'keys.styleTipDesktop' : 'keys.styleTip')"><Selector id="kb-preset" :model-value="which" :items="segItems(layoutItems())" size="sm" activation="manual" inline button-height pairs-on-phone @update:model-value="pickLayout($event as Preset)" /></FieldRow>
    <FieldRow :label="t('keys.search')"><TextField id="kb-q" v-model="query" :placeholder="t('keys.searchPh')" autocomplete="off" width="medium" /></FieldRow>
  </SubCard>
  <SubCard class="kb-well">
    <div id="kb-list" class="kb-list"><KeysList v-model:map="map" :base="base" :query="query" /></div>
    <div class="glim-eyebrow kb-fixedhead">{{ t('keys.fixedHead') }}</div>
    <div class="kbd-grid"><div v-for="k in FIXED" :key="k"><span>{{ t(`keys.fixedKeys.${k}.label`) }}</span><Kbd>{{ t(`keys.fixedKeys.${k}.key`, { mod: zoomMod() }) }}</Kbd></div></div>
  </SubCard>
</template>
