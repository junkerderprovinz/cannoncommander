<script setup lang="ts">
// A passkey gets its name here, then the browser asks for the fingerprint, the PIN or the key itself.
import { onMounted, ref } from 'vue';
import { me } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import Frow from '@/ui/Frow.vue';
import { c, fail, stepFoot } from '@/features/config/shared';

const props = defineProps<{ win: WinEntry; taken: string[]; done: () => void }>();
const name = ref(''), waiting = ref(false);
const el = ref<HTMLInputElement | null>(null);

async function next() {
  const n = name.value.trim();
  if (!n) return fail(c('security.needPasskeyName'), el.value);
  if (props.taken.includes(n)) return fail(c('nameTaken'), el.value);
  waiting.value = true;
  stepFoot(props.win, [cancelBtn()]);
  try {
    await me.addPasskey(n);
  } catch {
    waiting.value = false;
    showName();
    return fail(c('security.passkeyFailed'), el.value);
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
    <Frow :label="c('sources.name')" :tip="c('security.passkeyNameTip')"><input ref="el" v-model="name" class="glim-input m" :placeholder="c('security.passkeyNamePh')" @keydown.enter="next"></Frow>
  </div>
</template>
