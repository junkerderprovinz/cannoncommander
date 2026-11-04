<script setup lang="ts">
// A path field with the picker button beside it.
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { pickFolder } from './picker';
import './texts';

defineProps<{ id?: string }>();
const model = defineModel<string>({ required: true });

async function pick() {
  const p = await pickFolder(model.value || '/mnt/user');
  if (p) model.value = p;
}
</script>

<template>
  <span class="a-ctl"><input :id="id" v-model="model" class="field w mono" spellcheck="false" autocomplete="off"><button class="btn icon" type="button" :data-tip="t('ui.pickFolder')" :aria-label="t('ui.pickFolder')" @click.stop="pick"><G n="IconFolderOpen" /></button></span>
</template>
