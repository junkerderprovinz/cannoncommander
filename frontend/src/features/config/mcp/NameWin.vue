<script setup lang="ts">
import { FieldRow, SubCard, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { ApiError } from '@/api/client';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, fail, stepFoot } from '@/features/config/shared';

const props = defineProps<{ win: WinEntry; name: string; save: (name: string) => Promise<unknown> }>();
const name = ref(props.name);
const el = ref<InstanceType<typeof TextField> | null>(null);

async function go() {
  const n = name.value.trim();
  if (!n) return fail(c('mcp.needName'), el.value?.input);
  if (n === props.name) return props.win.close();
  try {
    await props.save(n);
  } catch (e) {
    return fail(e instanceof ApiError ? e.message : c('notSaved'), el.value?.input);
  }
  props.win.close();
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: c('save'), g: 'IconCheck', primary: true, run: () => { go(); return false; } }]));
</script>

<template>
  <SubCard>
    <FieldRow :label="c('sources.name')"><TextField id="mcp-name" ref="el" v-model="name" :placeholder="c('mcp.namePh')" width="medium" maxlength="64" @keydown.enter="go" /></FieldRow>
  </SubCard>
</template>
