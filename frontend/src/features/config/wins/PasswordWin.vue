<script setup lang="ts">
import { FieldRow, PasswordField, SubCard } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { me, statusOf } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, fail, stepFoot } from '@/features/config/shared';
import { eyeWords } from '@/ui/kit';

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

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: c('security.changePw'), g: 'IconKey', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <SubCard>
    <FieldRow :label="c('security.current')"><PasswordField id="a-pw0" v-model="p0" width="medium" autocomplete="current-password" :text="eyeWords()" /></FieldRow>
    <FieldRow :label="c('security.newPw')" :hint="c('security.newPwTip')"><PasswordField id="a-pw1" v-model="p1" width="medium" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
    <FieldRow :label="c('users.repeat')"><PasswordField id="a-pw2" v-model="p2" width="medium" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
  </SubCard>
</template>
