<script setup lang="ts">
// A token is shown once, right after it is made, and the window stays until it is confirmed as copied.
import { onMounted, ref } from 'vue';
import { me, statusOf, type Token } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import CopyField from '../kit/CopyField.vue';
import Frow from '../kit/Frow.vue';
import { c, fail, setFoot } from '../kit/kit';
import Seg from '../kit/Seg.vue';

const props = defineProps<{ win: WinEntry; taken: string[]; done: () => void }>();
const name = ref(''), rights = ref<Token['rights']>('r'), expires = ref('365');
const secret = ref<string | null>(null);
const el = ref<HTMLInputElement | null>(null);

async function create() {
  const n = name.value.trim();
  if (!n) return fail(c('security.needTokenName'), el.value);
  if (props.taken.includes(n)) return fail(c('nameTaken'), el.value);
  let got: { secret: string };
  try {
    got = await me.createToken(n, rights.value, expires.value === 'never' ? null : Number(expires.value));
  } catch (e) {
    return fail(statusOf(e) === 409 ? c('nameTaken') : c('notSaved'), el.value);
  }
  secret.value = got.secret;
  props.win.locked = true;
  setFoot(props.win, [{ label: c('done'), g: 'IconCheck', primary: true, run: () => { props.win.locked = false; props.done(); toast(c('security.tokenCreatedToast', { name: n })); } }]);
}

onMounted(() => setFoot(props.win, [cancelBtn(), { label: c('security.create'), g: 'IconKey', primary: true, run: () => { create(); return false; } }]));
</script>

<template>
  <template v-if="secret">
    <div class="well a-text"><p>{{ c('security.copyNow') }}</p></div>
    <div class="well"><Frow :label="name"><CopyField :value="secret" /></Frow></div>
  </template>
  <div v-else class="well">
    <Frow :label="c('sources.name')" :tip="c('security.tokenNameTip')"><input ref="el" v-model="name" class="field m" :placeholder="c('security.tokenNamePh')"></Frow>
    <Frow :label="c('security.rights')" :tip="c('security.rightsTip')"><Seg v-model="rights" :options="[['r', c('security.read')], ['rw', c('security.readWrite')]]" /></Frow>
    <Frow :label="c('shares.expires')"><Seg v-model="expires" :options="[['30', c('general.days', { n: 30 })], ['365', c('security.year')], ['never', c('shares.never_')]]" /></Frow>
  </div>
</template>
