<script setup lang="ts">
// A token is shown once, right after it is made, and the window stays until it is confirmed as copied.
import { CopyField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { me, statusOf, type Token } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import Frow from '@/ui/Frow.vue';
import { c, fail, stepFoot } from '@/features/config/shared';
import Seg from '@/ui/Seg.vue';
import { copyWords } from '@/ui/kit';

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
  stepFoot(props.win, [{ label: c('done'), g: 'IconCheck', primary: true, run: () => { props.win.locked = false; props.done(); toast(c('security.tokenCreatedToast', { name: n })); } }]);
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: c('security.create'), g: 'IconKey', primary: true, run: () => { create(); return false; } }]));
</script>

<template>
  <template v-if="secret">
    <div class="well glim-well a-text"><p>{{ c('security.copyNow') }}</p></div>
    <div class="well glim-well"><Frow :label="name"><CopyField class="w" :value="secret" :label="name" :text="copyWords()" /></Frow></div>
  </template>
  <div v-else class="well glim-well">
    <Frow :label="c('sources.name')" :tip="c('security.tokenNameTip')"><input id="a-tkname" ref="el" v-model="name" class="glim-input m" :placeholder="c('security.tokenNamePh')"></Frow>
    <Frow :label="c('security.rights')" :tip="c('security.rightsTip')"><Seg v-model="rights" :options="[['r', c('security.read')], ['rw', c('security.readWrite')]]" /></Frow>
    <Frow :label="c('shares.expires')"><Seg v-model="expires" :options="[['30', c('general.days', { n: 30 })], ['365', c('security.year')], ['never', c('shares.never_')]]" /></Frow>
  </div>
</template>
