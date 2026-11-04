<script setup lang="ts">
// The tools' folder field with the picker beside it.
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { pickFolder } from '@/ui/picker';
import './kit';

defineProps<{ id: string }>();
const model = defineModel<string>({ required: true });
const emit = defineEmits<{ picked: [path: string] }>();

async function pick() {
  const p = await pickFolder(model.value.trim() || '/mnt/user');
  if (p) { model.value = p; emit('picked', p); }
}
</script>

<template>
  <span class="bk-pathf"><input :id="id" v-model="model" class="field grow mono" spellcheck="false" autocomplete="off"><button class="btn icon" type="button" :data-bk-pick="id" :data-tip="t('bk.pickFolder')" :aria-label="t('bk.pickFolder')" @click.stop="pick"><G n="IconFolderOpen" /></button></span>
</template>
