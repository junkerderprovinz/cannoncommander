<script setup lang="ts">
// A path to type and the picker beside it, for folders or, with exts, for files with one of those endings.
import { Button } from '@glimstone/vue';
import { ref } from 'vue';
import { exists } from '@/api/admin';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { pickFolder } from './picker';
import './texts';

const props = defineProps<{ id?: string; exts?: string[]; placeholder?: string }>();
const model = defineModel<string>({ required: true });
const emit = defineEmits<{ picked: [path: string] }>();
const input = ref<HTMLInputElement | null>(null);
defineExpose({ input });

async function pick() {
  const here = usePanels().T().path;
  const known = await exists(model.value.trim(), props.exts ? 'file' : 'dir');
  const start = known ? model.value.trim() : here.includes('::') ? '/mnt/user' : here;
  const p = await pickFolder({ start, exts: props.exts });
  input.value?.focus();
  if (!p) return;
  model.value = p;
  emit('picked', p);
}
</script>

<template>
  <span class="pathf"><input :id="id" ref="input" v-model="model" class="glim-input glim-input--mono" :placeholder="placeholder" spellcheck="false" autocomplete="off"><Button variant="icon" :label="exts ? t('ui.pickFile') : t('ui.pickFolder')" label-key="IconFolderOpen" :data-pickfor="id" @click.stop="pick" /></span>
</template>
