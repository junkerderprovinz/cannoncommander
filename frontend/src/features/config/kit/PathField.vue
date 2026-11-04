<script setup lang="ts">
// A path field with the picker button beside it.
import { ref } from 'vue';
import { exists } from '@/api/admin';
import { G } from '@/core/glyphs';
import { usePanels } from '@/stores/panels';
import { c } from './kit';
import { pickFolder } from './picker';

const props = defineProps<{ exts?: string[]; placeholder?: string; id?: string }>();
const model = defineModel<string>({ required: true });
const emit = defineEmits<{ picked: [path: string] }>();
const input = ref<HTMLInputElement | null>(null);
defineExpose({ input });

async function pick() {
  const here = usePanels().T().path;
  const kind = props.exts ? 'file' : 'dir';
  const start = (await exists(model.value, kind)) ? model.value : here.includes('::') ? '/mnt/user' : here;
  const p = await pickFolder({ title: props.exts ? c('picker.titleFile') : c('picker.titleFolder'), start, exts: props.exts ?? null });
  if (!p) return;
  model.value = p;
  emit('picked', p);
}
</script>

<template>
  <span class="a-ctl"><input :id="id" ref="input" v-model="model" class="field w mono" :placeholder="placeholder"><button class="btn icon" type="button" :data-tip="exts ? c('picker.titleFile') : c('picker.titleFolder')" @click.stop="pick"><G n="IconFolderOpen" /></button></span>
</template>
