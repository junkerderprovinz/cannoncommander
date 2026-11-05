<script setup lang="ts">
import { CopyField, InfoBubble } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { me, statusOf } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import Frow from '@/ui/Frow.vue';
import { c, fail, stepFoot } from '@/features/config/shared';
import { copyWords } from '@/ui/kit';
import CodesWin from './CodesWin.vue';

const props = defineProps<{ win: WinEntry; done: () => void }>();
const secret = ref(''), qr = ref(''), code = ref('');
const codes = ref<string[] | null>(null);
const el = ref<HTMLInputElement | null>(null);
const group4 = (s: string) => s.match(/.{1,4}/g)?.join(' ') ?? '';

async function next() {
  const v = code.value.replace(/\s/g, '');
  if (!/^\d{6}$/.test(v)) return fail(c('security.sixDigits'), el.value);
  let got: { codes: string[] };
  try {
    got = await me.totpConfirm(v);
  } catch (e) {
    return fail(statusOf(e) === 429 ? c('security.tooMany') : c('security.codeClock'), el.value);
  }
  codes.value = got.codes;
}

onMounted(async () => {
  stepFoot(props.win, [cancelBtn(), { label: c('next'), g: 'IconForward', primary: true, run: () => { next(); return false; } }]);
  const got = await me.totpBegin().catch(() => null);
  if (got) { secret.value = got.secret; qr.value = got.qr; }
});
</script>

<template>
  <CodesWin v-if="codes" :win="win" :codes="codes" :done="() => { props.done(); toast(c('security.totpIsOn')); }" />
  <div v-else class="well glim-well a-totp">
    <div class="a-qr" role="img" :aria-label="c('security.qr')" v-html="qr"></div>
    <div class="a-totp-side">
      <div class="eyebrow">{{ c('security.scan') }} <InfoBubble :tip="c('security.scanTip')" /></div>
      <Frow :label="c('security.typeIt')"><CopyField class="w" :value="group4(secret)" :label="c('security.typeIt')" :text="copyWords()" /></Frow>
      <Frow :label="c('security.appCode')" :tip="c('security.sixDigitsTip')"><input id="a-totp" ref="el" v-model="code" class="glim-input s glim-input--mono" inputmode="numeric" maxlength="6" autocomplete="one-time-code" placeholder="123456" @keydown.enter="next"></Frow>
    </div>
  </div>
</template>
