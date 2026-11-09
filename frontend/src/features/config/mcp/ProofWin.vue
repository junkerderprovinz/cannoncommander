<script setup lang="ts">
// A step that hands out more than reading: allowing a key to write, or replacing it. The owner shows they are at the
// keyboard with the password or a fresh sign-in, and a replaced key shows once, until it is confirmed as copied.
import { CopyField, EmptyState, FieldRow, PasswordField, SubCard } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { ApiError } from '@/api/client';
import { glyphComponent } from '@/core/glyphs';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, fail, stepFoot } from '@/features/config/shared';
import { copyWords, eyeWords } from '@/ui/kit';
import { staleSignIn } from '../proof';
import { signInFor, type Step } from './again';

const props = defineProps<{
  win: WinEntry; text: string[]; verb: string; glyph: string; password: boolean; step: Step;
  // Resolves to a key to show once, or to nothing.
  run: (password?: string) => Promise<string | void>;
  done: () => void;
}>();
const current = ref(''), secret = ref(''), stale = ref(false);
const field = () => props.win.el?.querySelector('#mcp-pw') ?? null;

async function go() {
  if (props.password && !current.value) return fail(c('security.needCurrent'), field());
  let key: string | void;
  try {
    key = await props.run(props.password ? current.value : undefined);
  } catch (e) {
    if (staleSignIn(e)) return showStale();
    return fail(e instanceof ApiError ? e.message : c('notSaved'), field());
  }
  if (!key) {
    props.done();
    props.win.close();
    return;
  }
  secret.value = key;
  props.win.locked = true;
  stepFoot(props.win, [{ label: c('done'), g: 'IconCheck', primary: true, run: () => { props.win.locked = false; props.done(); } }]);
}

function showStale() {
  stale.value = true;
  stepFoot(props.win, [cancelBtn(), { label: c('security.signInAgain'), g: 'IconSignIn', primary: true, run: () => { signInFor(props.step); return false; } }]);
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: props.verb, g: props.glyph, primary: true, run: () => { go(); return false; } }]));
</script>

<template>
  <template v-if="secret">
    <SubCard class="a-text"><p>{{ c('mcp.copyNow') }}</p></SubCard>
    <SubCard><FieldRow :label="c('mcp.newKey')"><CopyField width="grow" :value="secret" :label="c('mcp.newKey')" :text="copyWords()" /></FieldRow></SubCard>
  </template>
  <EmptyState v-else-if="stale" nested :glyph="glyphComponent('IconSignIn')" :title="c('security.staleTitle')" :hint="c('mcp.stale')" />
  <template v-else>
    <SubCard class="a-text"><p v-for="(p, k) in text" :key="k">{{ p }}</p></SubCard>
    <SubCard v-if="password">
      <FieldRow :label="c('security.current')" :hint="c('security.proofTip')"><PasswordField id="mcp-pw" v-model="current" width="medium" autocomplete="current-password" :text="eyeWords()" @keydown.enter="go" /></FieldRow>
    </SubCard>
  </template>
</template>
