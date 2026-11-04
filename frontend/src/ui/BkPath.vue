<script setup lang="ts">
// The tool windows' folder field: the path to type and the picker beside it.
import { ref } from 'vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { pickFolder } from './picker';
import './texts';

defineProps<{ id: string }>();
const model = defineModel<string>({ required: true });
const input = ref<HTMLInputElement | null>(null);
defineExpose({ input });

async function pick() {
  const p = await pickFolder(model.value.trim() || '/mnt/user');
  if (p) model.value = p;
  input.value?.focus();
}
</script>

<template>
  <span class="bk-pathf"><input :id="id" ref="input" v-model="model" class="field grow mono" spellcheck="false" autocomplete="off"><button class="btn icon" type="button" :data-bk-pick="id" :data-tip="t('ui.pickFolder')" :aria-label="t('ui.pickFolder')" @click.stop="pick"><G n="IconFolderOpen" /></button></span>
</template>
