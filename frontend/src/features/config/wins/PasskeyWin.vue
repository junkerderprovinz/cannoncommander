<script setup lang="ts">
// A passkey gets its name here, then the browser asks for the fingerprint, the PIN or the key itself.
import { EmptyState, FieldRow, PasswordField, SubCard, TextField } from '@glimstone/vue';
import { nextTick, onMounted, ref } from 'vue';
import { me } from '@/api/admin';
import { glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, fail, stepFoot } from '@/features/config/shared';
import { eyeWords } from '@/ui/kit';
import { proofRefused, signInAgain, staleSignIn } from '../proof';

const props = defineProps<{ win: WinEntry; taken: string[]; password: boolean; done: () => void }>();
const name = ref(''), current = ref(''), waiting = ref(false), stale = ref(false);
const el = ref<InstanceType<typeof TextField> | null>(null);
const pwField = () => props.win.el?.querySelector('#a-pkpw') ?? null;

async function next() {
  const n = name.value.trim();
  if (!n) return fail(c('security.needPasskeyName'), el.value?.input);
  if (props.taken.includes(n)) return fail(c('nameTaken'), el.value?.input);
  if (props.password && !current.value) return fail(c('security.needCurrent'), pwField());
  waiting.value = true;
  stepFoot(props.win, [cancelBtn()]);
  try {
    await me.addPasskey(n, props.password ? current.value : undefined);
  } catch (e) {
    waiting.value = false;
    if (staleSignIn(e)) return showStale();
    showName();
    await nextTick();
    const refused = props.password ? proofRefused(e) : null;
    if (refused) return fail(refused, pwField());
    return fail(c('security.passkeyFailed'), el.value?.input);
  }
  if (!props.win.el?.isConnected) return;
  props.win.close();
  toast(c('security.passkeyAdded', { name: n }));
  props.done();
}
const showName = () => stepFoot(props.win, [cancelBtn(), { label: c('next'), g: 'IconForward', primary: true, run: () => { next(); return false; } }]);
function showStale() {
  stale.value = true;
  stepFoot(props.win, [cancelBtn(), { label: c('security.signInAgain'), g: 'IconSignIn', primary: true, run: () => { signInAgain('passkey'); return false; } }]);
}
onMounted(showName);
</script>

<template>
  <EmptyState v-if="waiting" nested :glyph="glyphComponent('IconKey')" :title="c('security.confirmPasskey')" />
  <EmptyState v-else-if="stale" nested :glyph="glyphComponent('IconSignIn')" :title="c('security.staleTitle')" :hint="c('security.stalePasskey')" />
  <SubCard v-else>
    <FieldRow :label="c('sources.name')" :hint="c('security.passkeyNameTip')"><TextField ref="el" v-model="name" :placeholder="c('security.passkeyNamePh')" width="medium" @keydown.enter="next" /></FieldRow>
    <FieldRow v-if="password" :label="c('security.current')" :hint="c('security.proofTip')"><PasswordField id="a-pkpw" v-model="current" width="medium" autocomplete="current-password" :text="eyeWords()" @keydown.enter="next" /></FieldRow>
  </SubCard>
</template>
