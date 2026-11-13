<script setup lang="ts">
import { FieldRow, PasswordField, SubCard } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { ApiError } from '@/api/client';
import { me, statusOf } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, cn, fail, stepFoot } from '@/features/config/shared';
import { serverText } from '@/i18n';
import { useUser } from '@/stores/user';
import { eyeWords } from '@/ui/kit';

// first sets the password of an account that has none, remove takes it off again.
const props = defineProps<{ win: WinEntry; done: () => void; mode?: 'first' | 'change' | 'remove' }>();
const mode = props.mode ?? 'change';
const p0 = ref(''), p1 = ref(''), p2 = ref('');
const at = (sel: string) => props.win.el?.querySelector(sel) ?? null;

function refused(e: unknown) {
  const status = statusOf(e);
  const said = e instanceof ApiError ? serverText(e.problem?.code, e.problem?.params) : undefined;
  if (status === 429) return fail(c('security.tooMany'), at('#a-pw0'));
  if (status === 403) return fail(c('security.wrongCurrent'), at('#a-pw0'));
  return fail(said ?? c('notSaved'), at(mode === 'first' ? '#a-pw1' : '#a-pw0'));
}

async function save() {
  if (mode !== 'first' && !p0.value) return fail(c('security.needCurrent'), at('#a-pw0'));
  if (mode === 'remove') {
    try {
      await me.removePassword(p0.value);
    } catch (e) {
      return refused(e);
    }
    useUser().me.signIn = false;
    props.win.close();
    toast(c('security.pwRemoved'));
    return props.done();
  }
  if (p1.value.length < 12) return fail(c('security.newShort'), at('#a-pw1'));
  if (mode === 'change' && p1.value === p0.value) return fail(c('security.sameAsOld'), at('#a-pw1'));
  if (p1.value !== p2.value) return fail(c('security.newDiffer'), at('#a-pw2'));
  let got: { ended: number };
  try {
    got = await me.password(p0.value, p1.value);
  } catch (e) {
    return refused(e);
  }
  props.win.close();
  if (mode === 'first') {
    useUser().me.signIn = true;
    toast(c('security.pwSetDone'));
  } else toast(got.ended ? cn(got.ended, 'security.pwChangedOthers') : c('security.pwChanged'));
  props.done();
}

const verb = { first: ['security.setPw', 'IconKey'], change: ['security.changePw', 'IconKey'], remove: ['security.removePw', 'IconTrash'] }[mode];
onMounted(() => stepFoot(props.win, [cancelBtn(), { label: c(verb[0]!), g: verb[1], primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <SubCard v-if="mode === 'remove'" class="a-text"><p>{{ c('security.removePwText') }}</p></SubCard>
  <SubCard>
    <FieldRow v-if="mode !== 'first'" :label="c('security.current')"><PasswordField id="a-pw0" v-model="p0" width="medium" autocomplete="current-password" :text="eyeWords()" /></FieldRow>
    <template v-if="mode !== 'remove'">
      <FieldRow :label="c('security.newPw')" :hint="c('security.newPwTip')"><PasswordField id="a-pw1" v-model="p1" width="medium" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
      <FieldRow :label="c('users.repeat')"><PasswordField id="a-pw2" v-model="p2" width="medium" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
    </template>
  </SubCard>
</template>
