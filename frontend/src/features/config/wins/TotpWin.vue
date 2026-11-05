<script setup lang="ts">
import { CopyField, FieldRow, InfoBubble, SubCard, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { me, statusOf } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, fail, stepFoot } from '@/features/config/shared';
import { copyWords } from '@/ui/kit';
import CodesWin from './CodesWin.vue';

const props = defineProps<{ win: WinEntry; done: () => void }>();
const secret = ref(''), qr = ref(''), code = ref('');
const codes = ref<string[] | null>(null);
const el = ref<InstanceType<typeof TextField> | null>(null);
const group4 = (s: string) => s.match(/.{1,4}/g)?.join(' ') ?? '';

async function next() {
  const v = code.value.replace(/\s/g, '');
  if (!/^\d{6}$/.test(v)) return fail(c('security.sixDigits'), el.value?.input);
  let got: { codes: string[] };
  try {
    got = await me.totpConfirm(v);
  } catch (e) {
    return fail(statusOf(e) === 429 ? c('security.tooMany') : c('security.codeClock'), el.value?.input);
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
  <SubCard v-else class="a-totp">
    <div class="a-qr" role="img" :aria-label="c('security.qr')" v-html="qr"></div>
    <div class="a-totp-side">
      <div class="glim-eyebrow">{{ c('security.scan') }} <InfoBubble :tip="c('security.scanTip')" /></div>
      <FieldRow :label="c('security.typeIt')"><CopyField width="grow" :value="group4(secret)" :label="c('security.typeIt')" :text="copyWords()" /></FieldRow>
      <FieldRow :label="c('security.appCode')" :hint="c('security.sixDigitsTip')"><TextField id="a-totp" ref="el" v-model="code" inputmode="numeric" maxlength="6" autocomplete="one-time-code" placeholder="123456" width="short" mono @keydown.enter="next" /></FieldRow>
    </div>
  </SubCard>
</template>
