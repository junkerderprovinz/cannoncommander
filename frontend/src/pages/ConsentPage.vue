<script setup lang="ts">
// The answer to a client that signs in through OAuth. The page shows who asks, as which account, where the answer
// goes and whether the client may write, and nothing leaves it before Allow or Deny. Allow makes a lasting way in,
// so it asks for the password, or for an account without one a fresh sign-in, as making a key does. A request the
// server refuses stays here as a sentence: its return address is whatever the client registered, which can be
// anybody's.
import { Button, Card, EmptyState, FieldRow, PasswordField, SubCard, Toggle } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { auth } from '@/api/auth';
import { ApiError } from '@/api/client';
import { mcp, type McpConsent } from '@/api/mcp';
import { addMessages } from '@/i18n';
import de from '@/features/config/de.json';
import en from '@/features/config/en.json';
import { glyphComponent } from '@/core/glyphs';
import { proofRefused, staleSignIn } from '@/features/config/proof';
import { c, fail } from '@/features/config/shared';
import { eyeWords } from '@/ui/kit';
import ClientMark from '@/features/config/mcp/ClientMark.vue';
import { clientById, OTHER_CLIENT } from '@/features/config/mcp/clients';
import { consentQuery, forgetConsentQuery, keepConsentQuery, returnTo } from './consent';

addMessages('config', { de, en });

// Allow stays off for a moment after the page shows or comes back to the front, so the second click of a double click
// meant for something else cannot land on it.
const PAUSE = 1000;

const info = ref<McpConsent | null>(null), problem = ref(''), leaving = ref('');
const write = ref(false), busy = ref(false), armed = ref(false), current = ref(''), stale = ref(false);
const pwField = () => document.querySelector('#mcp-consent-pw');
const known = computed(() => clientById(info.value?.client.known));
const name = computed(() => known.value?.name ?? (info.value?.client.name || c('mcp.consent.unnamed')));
const refusal = (e: unknown) => (e instanceof ApiError && e.problem?.code ? e.message : c('mcp.consent.failed'));

let timer: ReturnType<typeof setTimeout> | undefined;
function pause() {
  armed.value = false;
  clearTimeout(timer);
  timer = setTimeout(() => { armed.value = true; }, PAUSE);
}

async function answer(allow: boolean) {
  if (!info.value) return;
  const proof = allow && info.value.password;
  if (proof && !current.value) return fail(c('security.needCurrent'), pwField());
  busy.value = true;
  try {
    const { redirect } = await mcp.answer(info.value.ticket, allow, write.value, proof ? current.value : undefined);
    forgetConsentQuery();
    leaving.value = new URL(redirect).host;
    returnTo(redirect);
  } catch (e) {
    if (allow && staleSignIn(e)) {
      stale.value = true;
      return;
    }
    const refused = proof ? proofRefused(e) : null;
    if (refused) return fail(refused, pwField());
    info.value = null;
    problem.value = refusal(e);
  } finally {
    busy.value = false;
  }
}

// The provider sends the person back to this page, which takes the client's query along through the trip.
async function signInAgain() {
  keepConsentQuery();
  const url = (await auth.options().catch(() => null))?.oidc?.url ?? 'api/auth/oidc/login';
  location.assign(url + '?redirect=' + encodeURIComponent(location.pathname));
}

onMounted(async () => {
  addEventListener('focus', pause);
  document.addEventListener('visibilitychange', pause);
  try {
    info.value = await mcp.consent(consentQuery());
    pause();
  } catch (e) {
    problem.value = refusal(e);
  }
});
onBeforeUnmount(() => {
  clearTimeout(timer);
  removeEventListener('focus', pause);
  document.removeEventListener('visibilitychange', pause);
});
</script>

<template>
  <main class="consent">
    <Card class="consent-card" :title="c('mcp.consent.title')">
      <p v-if="leaving" role="status" class="sub">{{ c('mcp.consent.returning', { host: leaving }) }}</p>
      <SubCard v-if="problem" class="a-text" role="alert" data-consent="problem"><p>{{ problem }}</p></SubCard>
      <template v-if="info && !leaving">
        <div class="mcp-who">
          <span class="mcp-row-mark big"><ClientMark :client="known ?? OTHER_CLIENT" /></span>
          <!-- Nothing here is cut short: a lookalike host differs in the part an ellipsis would hide. -->
          <span><b>{{ name }}</b><small class="mono">{{ info.client.redirectHost }}</small></span>
        </div>
        <p>{{ c('mcp.consent.asks', { name, account: info.account }) }}</p>
        <p class="sub">{{ c('mcp.consent.reads') }}</p>
        <p class="sub">{{ known ? c('mcp.consent.known', { name: known.name }) : c('mcp.consent.unverified') }}</p>
        <SubCard v-if="info.client.loopback" class="a-text"><p>{{ c('mcp.consent.loopback') }}</p></SubCard>
        <SubCard v-if="info.limitReached" class="a-text" data-consent="full"><p>{{ c('mcp.consent.full', { n: info.grantLimit }) }}</p></SubCard>
        <FieldRow :label="c('mcp.write')" :hint="c('mcp.consent.writeTip')"><Toggle v-model="write" :disabled="busy" data-consent="write" /></FieldRow>
        <FieldRow v-if="info.password" :label="c('security.current')" :hint="c('security.proofTip')"><PasswordField id="mcp-consent-pw" v-model="current" width="medium" autocomplete="current-password" :text="eyeWords()" @keydown.enter="armed && answer(true)" /></FieldRow>
        <template v-if="stale">
          <EmptyState nested :glyph="glyphComponent('IconSignIn')" :title="c('security.staleTitle')" :hint="c('mcp.stale')" />
          <div class="consent-answer"><Button :label="c('security.signInAgain')" label-key="IconSignIn" tone="accent" data-consent="signin" @click="signInAgain" /></div>
        </template>
        <p class="sub">{{ c('mcp.consent.returnsTo', { host: info.client.redirectHost }) }}</p>
        <div class="consent-answer">
          <Button :label="c('mcp.consent.deny')" label-key="IconClose" :disabled="busy" data-consent="deny" @click="answer(false)" />
          <Button :label="c('mcp.consent.allow')" label-key="IconCheck" tone="accent" :disabled="busy || info.limitReached || !armed" data-consent="allow" @click="answer(true)" />
        </div>
      </template>
    </Card>
  </main>
</template>
