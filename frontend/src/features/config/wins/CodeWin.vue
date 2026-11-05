<script setup lang="ts">
// Asks for a code from the app, or one of the recovery codes, before two-factor login changes.
import { onMounted, ref } from 'vue';
import { cancelBtn, type WinEntry } from '@/core/windows';
import Frow from '@/ui/Frow.vue';
import { c, fail, stepFoot } from '@/features/config/shared';

const props = defineProps<{ win: WinEntry; verb: string; glyph: string; submit: (code: string) => Promise<void> }>();
const code = ref('');
const el = ref<HTMLInputElement | null>(null);

async function go() {
  const v = code.value.trim();
  if (!/^\d{6}$/.test(v) && !/^[A-Z0-9]{4}-[A-Z0-9]{4}$/i.test(v)) return fail(c('security.needCode'), el.value);
  const ok = await props.submit(v).then(() => true, () => false);
  if (!ok) return fail(c('security.codeWrong'), el.value);
  props.win.close();
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: props.verb, g: props.glyph, primary: true, run: () => { go(); return false; } }]));
</script>

<template>
  <div class="well glim-well">
    <Frow :label="c('security.appCode')" :tip="c('security.appCodeTip')"><input id="a-code" ref="el" v-model="code" class="glim-input s glim-input--mono" inputmode="numeric" maxlength="9" autocomplete="one-time-code" placeholder="123456" @keydown.enter="go"></Frow>
  </div>
</template>
