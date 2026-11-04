<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { me, statusOf } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import Frow from '../kit/Frow.vue';
import { c, fail, setFoot } from '../kit/kit';
import PwField from '../kit/PwField.vue';

const props = defineProps<{ win: WinEntry; done: () => void }>();
const p0 = ref(''), p1 = ref(''), p2 = ref('');
const at = (sel: string) => props.win.el?.querySelector(sel) ?? null;

async function save() {
  if (!p0.value) return fail(c('security.needCurrent'), at('#a-pw0'));
  if (p1.value.length < 12) return fail(c('security.newShort'), at('#a-pw1'));
  if (p1.value === p0.value) return fail(c('security.sameAsOld'), at('#a-pw1'));
  if (p1.value !== p2.value) return fail(c('security.newDiffer'), at('#a-pw2'));
  let got: { ended: number };
  try {
    got = await me.password(p0.value, p1.value);
  } catch (e) {
    const status = statusOf(e);
    return fail(status === 429 ? c('security.tooMany') : status === 403 ? c('security.wrongCurrent') : c('notSaved'), at('#a-pw0'));
  }
  props.win.close();
  toast(got.ended ? c(got.ended === 1 ? 'security.pwChangedOne' : 'security.pwChangedMany', { n: got.ended }) : c('security.pwChanged'));
  props.done();
}

onMounted(() => setFoot(props.win, [cancelBtn(), { label: c('security.changePw'), g: 'IconKey', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <div class="well">
    <Frow :label="c('security.current')"><PwField id="a-pw0" v-model="p0" size="m" autocomplete="current-password" /></Frow>
    <Frow :label="c('security.newPw')" :tip="c('security.newPwTip')"><PwField id="a-pw1" v-model="p1" size="m" /></Frow>
    <Frow :label="c('users.repeat')"><PwField id="a-pw2" v-model="p2" size="m" /></Frow>
  </div>
</template>
