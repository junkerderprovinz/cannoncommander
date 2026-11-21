<script setup lang="ts">
// The way in: GlimStone's sign-in card on the kit's sign-in. Where several accounts sign in, the name stands above the
// password, and the sign-in service that is on stands under Sign in. Passkeys come through the browser's offer in the
// password field.
import { SignInCard, SignInField, TextField, type SignInCardService } from '@glimstone/vue';
import { signInCardHandlers, signInServiceMark, takeSignInCode, type SignInOffer, type SignInRemoved, type SignInVisitor } from '@glimstone/core';
import { IDENTITY_PROVIDERS } from '@glimstone/core/identityProviders';
import { computed, onMounted, ref } from 'vue';
import logo from '@/assets/brand/cannoncommander.svg';
import { toast } from '@/core/toast';
import { t } from '@/i18n';
import { leaveNote, session, takeNote } from '.';
import { signIn } from './kit';
import { leaveFor, reload } from './leave';
import { arrivedWords, cardText, errorWords, resetNote } from './words';

// overlay is set when a session ran out while the app was open, so the app stays as it is behind the card.
const props = defineProps<{ overlay?: boolean; was?: string }>();
const s = (key: string, params?: Record<string, string | number>) => t('signin.' + key, params);

const CLOSED: SignInOffer = { open: false, owner: false, minPassword: 12, signedIn: false, passkeys: false, services: [] };
// The service of config.yaml starts on the paths its provider was set up with.
const CONFIG_SERVICE = 'config';

const offer = ref<SignInOffer | null>(null);
const name = ref(props.was ?? '');
const note = ref(takeNote() || (props.overlay ? s('again') : ''));
const code = takeSignInCode();
const refused = code ? ({ ...errorWords(), ...arrivedWords() } as Record<string, string | undefined>)[code] ?? s('failed') : undefined;

const text = computed(() => cardText(offer.value?.minPassword ?? 12));
const labelKeys = { signIn: 'IconSignIn', forgot: 'IconForgotPassword', back: 'IconBack', setPassword: 'IconKey', service: 'IconSignIn' };
const many = computed(() => !offer.value?.owner);
const first = computed(() => offer.value?.services[0]);
const service = computed<SignInCardService | undefined>(() => {
  const f = first.value;
  if (!f) return undefined;
  const mark = signInServiceMark(IDENTITY_PROVIDERS, f.provider);
  return mark ? { name: f.name, mark } : { name: f.name };
});

onMounted(async () => {
  const r = await signIn.options();
  offer.value = r.ok ? r.value : CLOSED;
});

function finish(visitor: SignInVisitor | undefined) {
  const who = visitor?.name || name.value;
  // A session that ran out comes back to the app as it was, unless someone else signs in.
  if (props.overlay && (!props.was || who === props.was)) {
    session.out = false;
    toast(s('hello', { name: who }));
    return;
  }
  leaveNote(s('hello', { name: who }));
  reload();
}

function reset(removed: SignInRemoved) {
  leaveNote(resetNote(removed));
  reload();
}

const handlers = computed(() => {
  const h = signInCardHandlers(signIn, {
    words: { errors: errorWords(), failed: s('failed') },
    onSignedIn: finish,
    onReset: reset,
    name: () => (many.value ? name.value.trim() : undefined),
  });
  return {
    ...h,
    onSignIn: (input: Parameters<typeof h.onSignIn>[0]) =>
      many.value && input.code === undefined && !name.value.trim() ? Promise.resolve({ error: s('needBoth') }) : h.onSignIn(input),
  };
});

function toService() {
  const f = first.value;
  if (!f) return;
  const back = location.pathname;
  leaveFor(f.id === CONFIG_SERVICE ? `${signIn.root}api/auth/oidc/login?redirect=${encodeURIComponent(back)}` : signIn.serviceAddress(f.id, back));
}
</script>

<template>
  <SignInCard
    v-if="offer"
    class="cl"
    :class="{ over: overlay }"
    :logo="logo"
    app-name="CannonCommander"
    :text="text"
    reset
    :min-password="offer.minPassword"
    :note="note"
    :error="refused"
    :service="service"
    :on-service="toService"
    :label-keys="labelKeys"
    id-prefix="cl"
    :on-sign-in="handlers.onSignIn"
    :on-passkey-offer="offer.passkeys ? handlers.onPasskeyOffer : undefined"
    :on-ask-code="handlers.onAskCode"
    :on-set-password="handlers.onSetPassword"
  >
    <template v-if="many" #fields>
      <SignInField :label="s('name')" for="cl-user">
        <TextField id="cl-user" v-model="name" autocomplete="username" autocapitalize="off" spellcheck="false" />
      </SignInField>
    </template>
  </SignInCard>
</template>
