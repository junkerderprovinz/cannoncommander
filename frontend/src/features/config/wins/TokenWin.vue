<script setup lang="ts">
// A token is shown once, right after it is made, and the window stays until it is confirmed as copied.
import { CopyField, FieldRow, Selector, SubCard, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { me, statusOf, type Token } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, fail, stepFoot } from '@/features/config/shared';
import { copyWords, segItems } from '@/ui/kit';

const props = defineProps<{ win: WinEntry; taken: string[]; done: () => void }>();
const name = ref(''), rights = ref<Token['rights']>('r'), expires = ref('365');
const secret = ref<string | null>(null);
const el = ref<InstanceType<typeof TextField> | null>(null);

async function create() {
  const n = name.value.trim();
  if (!n) return fail(c('security.needTokenName'), el.value?.input);
  if (props.taken.includes(n)) return fail(c('nameTaken'), el.value?.input);
  let got: { secret: string };
  try {
    got = await me.createToken(n, rights.value, expires.value === 'never' ? null : Number(expires.value));
  } catch (e) {
    return fail(statusOf(e) === 409 ? c('nameTaken') : c('notSaved'), el.value?.input);
  }
  secret.value = got.secret;
  props.win.locked = true;
  stepFoot(props.win, [{ label: c('done'), g: 'IconCheck', primary: true, run: () => { props.win.locked = false; props.done(); toast(c('security.tokenCreatedToast', { name: n })); } }]);
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: c('security.create'), g: 'IconKey', primary: true, run: () => { create(); return false; } }]));
</script>

<template>
  <template v-if="secret">
    <SubCard class="a-text"><p>{{ c('security.copyNow') }}</p></SubCard>
    <SubCard><FieldRow :label="name"><CopyField width="grow" :value="secret" :label="name" :text="copyWords()" /></FieldRow></SubCard>
  </template>
  <SubCard v-else>
    <FieldRow :label="c('sources.name')" :hint="c('security.tokenNameTip')"><TextField id="a-tkname" ref="el" v-model="name" :placeholder="c('security.tokenNamePh')" width="medium" /></FieldRow>
    <FieldRow :label="c('security.rights')" :hint="c('security.rightsTip')"><Selector v-model="rights" :items="segItems([['r', c('security.read')], ['rw', c('security.readWrite')]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="c('shares.expires')"><Selector v-model="expires" :items="segItems([['30', c('general.days', { n: 30 })], ['365', c('security.year')], ['never', c('shares.never_')]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
  </SubCard>
</template>
