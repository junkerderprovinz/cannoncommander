<script setup lang="ts">
// A passkey gets its name here, then the browser asks for the fingerprint, the PIN or the key itself.
import { FieldRow, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { me } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, fail, stepFoot } from '@/features/config/shared';

const props = defineProps<{ win: WinEntry; taken: string[]; done: () => void }>();
const name = ref(''), waiting = ref(false);
const el = ref<InstanceType<typeof TextField> | null>(null);

async function next() {
  const n = name.value.trim();
  if (!n) return fail(c('security.needPasskeyName'), el.value?.input);
  if (props.taken.includes(n)) return fail(c('nameTaken'), el.value?.input);
  waiting.value = true;
  stepFoot(props.win, [cancelBtn()]);
  try {
    await me.addPasskey(n);
  } catch {
    waiting.value = false;
    showName();
    return fail(c('security.passkeyFailed'), el.value?.input);
  }
  if (!props.win.el?.isConnected) return;
  props.win.close();
  toast(c('security.passkeyAdded', { name: n }));
  props.done();
}
const showName = () => stepFoot(props.win, [cancelBtn(), { label: c('next'), g: 'IconForward', primary: true, run: () => { next(); return false; } }]);
onMounted(showName);
</script>

<template>
  <div v-if="waiting" class="empty"><G n="IconKey" /><div>{{ c('security.confirmPasskey') }}</div></div>
  <div v-else class="well glim-well">
    <FieldRow :label="c('sources.name')" :hint="c('security.passkeyNameTip')"><TextField ref="el" v-model="name" :placeholder="c('security.passkeyNamePh')" width="medium" @keydown.enter="next" /></FieldRow>
  </div>
</template>
