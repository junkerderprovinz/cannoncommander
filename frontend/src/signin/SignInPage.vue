<script setup lang="ts">
// The way in: GlimStone's sign-in card with the name and "Stay signed in" above the password and single sign-on under
// Sign in. The steps of CannonCommander's own run in the same card: a password of its own after the first start,
// setting up two factor, the recovery codes and the page after a reset.
import { Button, FieldRow, PasswordField, SignInCard, SignInField, SubCard, TextField, Toggle, type SignInAnswer, type SignInText } from '@glimstone/vue';
import { shake } from '@glimstone/core';
import { computed, nextTick, onMounted, ref } from 'vue';
import { auth, type SignIn, type SignInOptions, type TotpSetup } from '@/api/auth';
import { ApiError } from '@/api/client';
import type { Me } from '@/api/types';
import logo from '@/assets/brand/cannoncommander.svg';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { plural, t } from '@/i18n';
import { eyeWords } from '@/ui/kit';
import { leaveNote, session, takeNote } from '.';

// overlay is set when a session ran out while the app was open, so the app stays as it is behind the card.
const props = defineProps<{ overlay?: boolean; was?: string }>();
const s = (key: string, params?: Record<string, string | number>) => t('signin.' + key, params);

type StepName = 'card' | 'password' | 'totpSetup' | 'codes' | 'resetDone' | 'leaving';
const step = ref<StepName>('card');
const opts = ref<SignInOptions>({ password: true, passkey: false, oidc: null, server: '', version: '', minPassword: 12 });
const note = ref(''), err = ref(''), busy = ref(false);
const name = ref(props.was ?? ''), keep = ref(true);
const next1 = ref(''), next2 = ref(''), setupCode = ref('');
const challenge = ref(''), setup = ref<TotpSetup | null>(null), codes = ref<string[]>([]), signedIn = ref<Me | null>(null);
// A new key starts the card over, when a step ran out on the server.
const cardKey = ref(0);
const form = ref<HTMLFormElement | null>(null);

const text = computed<SignInText>(() => ({
  password: s('password'), signIn: s('signIn'), signingIn: s('signingIn'), passkey: s('passkey'), forgot: s('forgot'),
  code: s('codeLabel'), codeHint: s('codeTip'), codeInvalid: s('badCode'), back: s('back'),
  resetCode: s('logCode'), resetCodeTip: s('logCodeTip'), newPassword: s('newPassword'), confirmPassword: s('repeat'),
  setPassword: s('setPassword'), saving: s('saving'), mismatch: s('differ'), tooShort: plural(opts.value.minPassword, 'signin.short'),
  resetRejected: s('badReset'), failed: s('offline'), ...eyeWords(),
}));
const labelKeys = { signIn: 'IconSignIn', passkey: 'IconKey', forgot: 'IconForgotPassword', back: 'IconBack', setPassword: 'IconKey' };
const stepTitle = computed(() => ({ password: s('ownTitle'), totpSetup: s('setupTitle'), codes: s('codesTitle'), resetDone: s('resetDone') } as Partial<Record<StepName, string>>)[step.value]);

onMounted(async () => {
  note.value = takeNote() || (props.overlay ? s('again') : '');
  try { opts.value = await auth.options(); } catch { /* the defaults offer the password, which every server takes */ }
});

// The server's answers that mean something to the person at the keyboard. Anything else is a server that is gone.
function reason(e: unknown, map: Partial<Record<number, string>>): string {
  if (e instanceof ApiError) {
    if (e.status === 429) return plural(Math.max(1, Math.ceil((e.retryAfter ?? 900) / 60)), 'signin.tooMany');
    const text = map[e.status];
    if (text) return text;
  }
  return s('offline');
}
const ranOut = (e: unknown) => e instanceof ApiError && e.status === 410;

function restart(why: string) {
  note.value = why;
  err.value = '';
  step.value = 'card';
  cardKey.value++;
}

function go(next: StepName) {
  step.value = next;
  err.value = '';
  nextTick(() => form.value?.querySelector<HTMLElement>('input:not([type=hidden]), button[type=submit]')?.focus());
}

// A refused step of the page's own answers as the card does: the reason, a shake, and the field ready for a new try.
function refuse(why: string, field?: string) {
  err.value = why;
  nextTick(() => {
    const input = field ? form.value?.querySelector<HTMLInputElement>(field) : null;
    input?.focus();
    input?.select();
    const button = form.value?.querySelector<HTMLElement>('button[type=submit]');
    if (button) shake(button);
  });
}

function proceed(r: SignIn): SignInAnswer {
  challenge.value = r.challenge ?? '';
  if (r.next === 'totp') return { needCode: true };
  if (r.next === 'done') {
    if (r.codes?.length) {
      codes.value = r.codes;
      signedIn.value = r.me ?? null;
      go('codes');
    } else finish(r.me);
  } else if (r.next === 'password') {
    next1.value = next2.value = '';
    go('password');
  } else {
    setup.value = r.setup ?? null;
    setupCode.value = '';
    go('totpSetup');
  }
  return { ok: true };
}

function finish(me?: Me | null) {
  const who = me?.full || me?.name || name.value;
  // A session that ran out comes back to the app as it was, unless someone else signs in.
  if (props.overlay && me && (!props.was || me.name === props.was)) {
    session.out = false;
    toast(s('hello', { name: who }));
    return;
  }
  leaveNote(s('hello', { name: who }));
  location.reload();
}

async function signIn({ password, code }: { password: string; code?: string }): Promise<SignInAnswer> {
  if (code !== undefined) {
    try {
      return proceed(await auth.code(challenge.value, code));
    } catch (e) {
      if (ranOut(e)) {
        restart(s('stale'));
        return { ok: true };
      }
      // Asking for the code again empties the field for a fresh one.
      return { needCode: true, error: reason(e, { 400: s('badCode') }) };
    }
  }
  const n = name.value.trim();
  if (!n) return { error: s('needBoth') };
  try {
    return proceed(await auth.login(n, password, keep.value));
  } catch (e) {
    return { error: reason(e, { 401: s('wrong'), 400: s('needBoth'), 403: s('noPasswords') }) };
  }
}

async function passkey(): Promise<SignInAnswer> {
  try {
    return proceed(await auth.passkey(keep.value));
  } catch (e) {
    if (e instanceof ApiError) return { error: reason(e, { 401: s('passkeyFailed'), 409: s('passkeyOff') }) };
    // The browser's own window was closed or found no passkey, which needs no scolding.
    if (e instanceof DOMException && e.name === 'NotAllowedError') return {};
    return { error: s('passkeyFailed') };
  }
}

async function askCode() {
  const n = name.value.trim();
  if (!n) {
    nextTick(() => document.getElementById('cl-user')?.focus());
    return { error: s('needName') };
  }
  try {
    await auth.askReset(n);
  } catch (e) {
    return { error: reason(e, {}) };
  }
  toast(s('codeSent'));
  return { ok: true };
}

async function reset({ code, password }: { code: string; password: string }) {
  const plain = code.replace(/[\s-]/g, '');
  if (!/^[a-z0-9]{12}$/i.test(plain)) return { error: s('twelve') };
  try {
    signedIn.value = await auth.reset(plain, password);
    go('resetDone');
    return { ok: true };
  } catch (e) {
    return { error: reason(e, { 400: s('badReset') }) };
  }
}

async function attempt(fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true;
  try { await fn(); } finally { busy.value = false; }
}

function ownPassword() {
  const min = opts.value.minPassword;
  if (next1.value.length < min) return refuse(plural(min, 'signin.short'), '#cl-new');
  if (next1.value !== next2.value) return refuse(s('differ'), '#cl-new2');
  attempt(async () => {
    try {
      proceed(await auth.newPassword(challenge.value, next1.value));
    } catch (e) {
      if (ranOut(e)) return restart(s('stale'));
      refuse(reason(e, { 400: s('same') }), '#cl-new');
    }
  });
}

function confirmSetup() {
  const code = setupCode.value.replace(/\s/g, '');
  if (!/^\d{6}$/.test(code)) return refuse(s('sixDigits'), '#cl-setup-code');
  attempt(async () => {
    try {
      proceed(await auth.code(challenge.value, code));
    } catch (e) {
      if (ranOut(e)) return restart(s('stale'));
      setupCode.value = '';
      refuse(reason(e, { 400: s('badSetupCode') }), '#cl-setup-code');
    }
  });
}

function submitStep() {
  if (step.value === 'password') ownPassword();
  else if (step.value === 'totpSetup') confirmSetup();
  else if (step.value === 'codes' || step.value === 'resetDone') finish(signedIn.value);
}

function oidc() {
  if (!opts.value.oidc) return;
  step.value = 'leaving';
  location.assign(opts.value.oidc.url);
}
</script>

<template>
  <SignInCard
    :key="cardKey"
    class="cl"
    :class="{ over: overlay }"
    :logo="logo"
    app-name="CannonCommander"
    :text="text"
    :passkey="opts.passkey"
    reset
    :min-password="opts.minPassword"
    :note="note"
    :label-keys="labelKeys"
    id-prefix="cl"
    :on-sign-in="signIn"
    :on-passkey="passkey"
    :on-ask-code="askCode"
    :on-set-password="reset"
  >
    <template #fields>
      <SignInField :label="s('name')" for="cl-user">
        <TextField id="cl-user" v-model="name" autocomplete="username" autocapitalize="off" spellcheck="false" />
      </SignInField>
      <FieldRow :label="s('stay')" :hint="s('stayTip')"><Toggle id="cl-keep" v-model="keep" /></FieldRow>
    </template>
    <template #actions>
      <Button v-if="opts.oidc" :label="s('oidc', { name: opts.oidc.label })" label-key="IconExternalLink" data-cl="oidc" @click="oidc" />
    </template>

    <template v-if="step === 'leaving'" #step>
      <div class="cl-wait" role="status"><span class="cl-spin"></span><div>{{ s('leaving', { name: opts.oidc?.label ?? '' }) }}</div></div>
    </template>
    <template v-else-if="step !== 'card'" #step>
      <form ref="form" class="glim-signin-form" novalidate @submit.prevent="submitStep">
        <h2 class="cl-title">{{ stepTitle }}</h2>

        <template v-if="step === 'password'">
          <p class="cl-text">{{ s('ownText') }}</p>
          <SignInField :label="s('newPassword')" for="cl-new">
            <PasswordField id="cl-new" v-model="next1" autocomplete="new-password" :placeholder="plural(opts.minPassword, 'signin.minChars')" :text="eyeWords()" />
          </SignInField>
          <SignInField :label="s('repeat')" for="cl-new2">
            <PasswordField id="cl-new2" v-model="next2" autocomplete="new-password" :text="eyeWords()" />
          </SignInField>
        </template>

        <template v-else-if="step === 'totpSetup'">
          <p class="cl-text">{{ s('setupText') }}</p>
          <template v-if="setup">
            <!-- The server draws the QR code as SVG from the secret it just made. -->
            <div class="cl-qr" v-html="setup.qr"></div>
            <div class="cl-key"><span class="muted">{{ s('setupKey') }}</span><span class="mono">{{ setup.secret }}</span></div>
          </template>
          <SignInField :label="s('codeLabel')" for="cl-setup-code">
            <TextField id="cl-setup-code" v-model="setupCode" class="glim-signin-code" inputmode="numeric" autocomplete="one-time-code" spellcheck="false" placeholder="000000" />
          </SignInField>
        </template>

        <template v-else-if="step === 'codes'">
          <p class="cl-text">{{ s('codesText') }}</p>
          <SubCard class="cl-codes mono glim-num"><span v-for="c in codes" :key="c">{{ c }}</span></SubCard>
        </template>

        <SubCard v-else :title="s('goneToo')" class="cl-gone">
          <div v-for="k in ['goneSessions', 'goneTwofa', 'goneTokens']" :key="k" class="cl-li"><G n="IconCheck" /><span>{{ s(k) }}</span></div>
        </SubCard>

        <p v-if="err" class="glim-signin-error" role="alert">{{ err }}</p>
        <template v-if="step === 'password' || step === 'totpSetup'">
          <Button :label="step === 'password' ? s('setPassword') : s('confirm')" :label-key="step === 'password' ? 'IconKey' : 'IconConfirm'" tone="accent" type="submit" :disabled="busy" />
          <Button tone="subtle" :label="s('back')" label-key="IconBack" data-cl="back" @click="restart('')" />
        </template>
        <Button v-else :label="s('toFiles')" label-key="IconForward" tone="accent" type="submit" />
      </form>
    </template>
  </SignInCard>
</template>
