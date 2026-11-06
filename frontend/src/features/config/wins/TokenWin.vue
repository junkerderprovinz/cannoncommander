<script setup lang="ts">
// A token is shown once, right after it is made, and the window stays until it is confirmed as copied.
import { CopyField, EmptyState, FieldRow, PasswordField, Selector, SubCard, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { me, statusOf, type Token } from '@/api/admin';
import { glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, cn, fail, stepFoot } from '@/features/config/shared';
import { copyWords, eyeWords, segItems } from '@/ui/kit';
import { proofRefused, signInAgain, staleSignIn } from '../proof';

const props = defineProps<{ win: WinEntry; taken: string[]; password: boolean; done: () => void }>();
const name = ref(''), rights = ref<Token['rights']>('r'), expires = ref('365'), current = ref('');
const secret = ref<string | null>(null), stale = ref(false);
const el = ref<InstanceType<typeof TextField> | null>(null);
const pwField = () => props.win.el?.querySelector('#a-tkpw') ?? null;

async function create() {
  const n = name.value.trim();
  if (!n) return fail(c('security.needTokenName'), el.value?.input);
  if (props.taken.includes(n)) return fail(c('nameTaken'), el.value?.input);
  if (props.password && !current.value) return fail(c('security.needCurrent'), pwField());
  let got: { secret: string };
  try {
    got = await me.createToken(n, rights.value, expires.value === 'never' ? null : Number(expires.value), props.password ? current.value : undefined);
  } catch (e) {
    if (staleSignIn(e)) return showStale();
    const refused = props.password ? proofRefused(e) : null;
    if (refused) return fail(refused, pwField());
    return fail(statusOf(e) === 409 ? c('nameTaken') : c('notSaved'), el.value?.input);
  }
  secret.value = got.secret;
  props.win.locked = true;
  stepFoot(props.win, [{ label: c('done'), g: 'IconCheck', primary: true, run: () => { props.win.locked = false; props.done(); toast(c('security.tokenCreatedToast', { name: n })); } }]);
}

function showStale() {
  stale.value = true;
  stepFoot(props.win, [cancelBtn(), { label: c('security.signInAgain'), g: 'IconSignIn', primary: true, run: () => { signInAgain('token'); return false; } }]);
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: c('security.create'), g: 'IconKey', primary: true, run: () => { create(); return false; } }]));
</script>

<template>
  <template v-if="secret">
    <SubCard class="a-text"><p>{{ c('security.copyNow') }}</p></SubCard>
    <SubCard><FieldRow :label="name"><CopyField width="grow" :value="secret" :label="name" :text="copyWords()" /></FieldRow></SubCard>
  </template>
  <EmptyState v-else-if="stale" nested :glyph="glyphComponent('IconSignIn')" :title="c('security.staleTitle')" :hint="c('security.staleToken')" />
  <SubCard v-else>
    <FieldRow :label="c('sources.name')" :hint="c('security.tokenNameTip')"><TextField id="a-tkname" ref="el" v-model="name" :placeholder="c('security.tokenNamePh')" width="medium" /></FieldRow>
    <FieldRow :label="c('security.rights')" :hint="c('security.rightsTip')"><Selector v-model="rights" :items="segItems([['r', c('security.read')], ['rw', c('security.readWrite')]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="c('shares.expires')"><Selector v-model="expires" :items="segItems([['30', cn(30, 'general.days')], ['365', c('security.year')], ['never', c('shares.never_')]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow v-if="password" :label="c('security.current')" :hint="c('security.proofTip')"><PasswordField id="a-tkpw" v-model="current" width="medium" autocomplete="current-password" :text="eyeWords()" @keydown.enter="create" /></FieldRow>
  </SubCard>
</template>
