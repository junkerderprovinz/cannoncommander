<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { me } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import CopyField from '../kit/CopyField.vue';
import Frow from '../kit/Frow.vue';
import Info from '../kit/Info.vue';
import { c, fail, setFoot } from '../kit/kit';
import CodesWin from './CodesWin.vue';

const props = defineProps<{ win: WinEntry; done: () => void }>();
const secret = ref(''), qr = ref(''), code = ref('');
const codes = ref<string[] | null>(null);
const el = ref<HTMLInputElement | null>(null);
const group4 = (s: string) => s.match(/.{1,4}/g)?.join(' ') ?? '';

async function next() {
  const v = code.value.replace(/\s/g, '');
  if (!/^\d{6}$/.test(v)) return fail(c('security.sixDigits'), el.value);
  const got = await me.totpConfirm(v).catch(() => null);
  if (!got) return fail(c('security.codeClock'), el.value);
  codes.value = got.codes;
}

onMounted(async () => {
  setFoot(props.win, [cancelBtn(), { label: c('next'), g: 'IconForward', primary: true, run: () => { next(); return false; } }]);
  const got = await me.totpBegin().catch(() => null);
  if (got) { secret.value = got.secret; qr.value = got.qr; }
});
</script>

<template>
  <CodesWin v-if="codes" :win="win" :codes="codes" :done="() => { props.done(); toast(c('security.totpIsOn')); }" />
  <div v-else class="well a-totp">
    <div class="a-qr" role="img" :aria-label="c('security.qr')" v-html="qr"></div>
    <div class="a-totp-side">
      <div class="eyebrow">{{ c('security.scan') }} <Info :tip="c('security.scanTip')" /></div>
      <Frow :label="c('security.typeIt')"><CopyField :value="group4(secret)" /></Frow>
      <Frow :label="c('security.appCode')" :tip="c('security.sixDigitsTip')"><input id="a-totp" ref="el" v-model="code" class="field s mono" inputmode="numeric" maxlength="6" autocomplete="one-time-code" placeholder="123456" @keydown.enter="next"></Frow>
    </div>
  </div>
</template>
