<script setup lang="ts">
// The way in: name and password, then the code from the app or a recovery code, a passkey instead, a password of
// its own after the first start, and the reset with a code from the container log.
import { Button, Card, FieldRow, InfoBubble, PasswordField, SubCard, TextField, Toggle } from '@glimstone/vue';
import { computed, nextTick, onMounted, ref } from 'vue';
import { auth, type SignIn, type SignInOptions, type TotpSetup } from '@/api/auth';
import { ApiError } from '@/api/client';
import type { Me } from '@/api/types';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { t } from '@/i18n';
import { eyeWords } from '@/ui/kit';
import { leaveNote, session, takeNote } from '.';

// overlay is set when a session ran out while the app was open, so the app stays as it is behind the card.
const props = defineProps<{ overlay?: boolean; was?: string }>();
const s = (key: string, params?: Record<string, string | number>) => t('signin.' + key, params);

type StepName = 'pw' | 'totp' | 'password' | 'totpSetup' | 'codes' | 'reset' | 'resetDone' | 'leaving';
const step = ref<StepName>('pw');
const opts = ref<SignInOptions>({ password: true, passkey: false, oidc: null, server: '', version: '', minPassword: 12 });
const err = ref(''), note = ref(''), busy = ref(false);
const name = ref(''), pass = ref(''), keep = ref(true);
const digits = ref<string[]>(['', '', '', '', '', '']), recovery = ref(''), byRecovery = ref(false);
const next1 = ref(''), next2 = ref(''), logCode = ref('');
const challenge = ref(''), setup = ref<TotpSetup | null>(null), codes = ref<string[]>([]), signedIn = ref<Me | null>(null);
const form = ref<HTMLFormElement | null>(null);

const title = computed(() => ({
  pw: s('title'), totp: s('confirmTitle'), password: s('ownTitle'), totpSetup: s('setupTitle'), codes: s('codesTitle'),
  reset: s('resetTitle'), resetDone: s('resetDone'), leaving: s('title'),
})[step.value]);

onMounted(async () => {
  note.value = takeNote() || (props.overlay ? s('again') : '');
  if (props.was) name.value = props.was;
  focus();
  try { opts.value = await auth.options(); } catch { /* the defaults offer the password, which every server takes */ }
});

function focus(sel?: string) {
  nextTick(() => {
    const el = form.value?.querySelector<HTMLElement>(sel ?? (step.value === 'pw' && name.value ? '#cl-pass' : 'input:not([type=hidden]), button[type=submit]'));
    el?.focus();
  });
}
function go(next: StepName, sel?: string) {
  step.value = next;
  err.value = '';
  focus(sel);
}
function fail(text: string, sel?: string) {
  err.value = text;
  focus(sel);
}

// The server's answers that mean something to the person at the keyboard. Anything else is a server that is gone.
function explain(e: unknown, map: Partial<Record<number, string>>, sel?: string) {
  if (e instanceof ApiError) {
    if (e.status === 429) return fail(s('tooMany', { n: Math.max(1, Math.ceil((e.retryAfter ?? 900) / 60)) }), sel);
    if (e.status === 410) {
      pass.value = '';
      go('pw');
      return fail(s('stale'), '#cl-pass');
    }
    const text = map[e.status];
    if (text) return fail(text, sel);
  }
  fail(s('offline'), sel);
}

async function attempt(fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true;
  try { await fn(); } finally { busy.value = false; }
}

function proceed(r: SignIn) {
  challenge.value = r.challenge ?? '';
  if (r.next === 'done') {
    if (r.codes?.length) {
      codes.value = r.codes;
      signedIn.value = r.me ?? null;
      return go('codes');
    }
    return finish(r.me);
  }
  if (r.next === 'totp') {
    digits.value = ['', '', '', '', '', ''];
    recovery.value = '';
    return go('totp');
  }
  if (r.next === 'password') {
    next1.value = next2.value = '';
    return go('password');
  }
  setup.value = r.setup ?? null;
  digits.value = ['', '', '', '', '', ''];
  go('totpSetup');
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

function submit() {
  const run: Partial<Record<StepName, () => void>> = { pw: signIn, totp: sendCode, totpSetup: sendCode, password: ownPassword, codes: () => finish(signedIn.value), reset, resetDone: () => finish(signedIn.value) };
  run[step.value]?.();
}

function signIn() {
  const n = name.value.trim();
  if (!n || !pass.value) return fail(s('needBoth'), n ? '#cl-pass' : '#cl-user');
  attempt(async () => {
    try {
      proceed(await auth.login(n, pass.value, keep.value));
    } catch (e) {
      explain(e, { 401: s('wrong'), 400: s('needBoth'), 403: s('noPasswords') }, '#cl-pass');
    }
  });
}

function sendCode() {
  const recover = step.value === 'totp' && byRecovery.value;
  const code = recover ? recovery.value.trim() : digits.value.join('');
  if (!recover && code.length < 6) return fail(s('sixDigits'), '[data-d="0"]');
  if (recover && !code) return fail(s('badCode'), '#cl-recovery');
  attempt(async () => {
    try {
      proceed(await auth.code(challenge.value, code));
    } catch (e) {
      digits.value = ['', '', '', '', '', ''];
      explain(e, { 400: step.value === 'totpSetup' ? s('badSetupCode') : s('badCode') }, recover ? '#cl-recovery' : '[data-d="0"]');
    }
  });
}

function checkNew(): boolean {
  const min = opts.value.minPassword;
  if (next1.value.length < min) fail(s('short', { n: min }), '#cl-new');
  else if (next1.value !== next2.value) fail(s('differ'), '#cl-new2');
  else return true;
  return false;
}

function ownPassword() {
  if (!checkNew()) return;
  attempt(async () => {
    try {
      proceed(await auth.newPassword(challenge.value, next1.value));
    } catch (e) {
      explain(e, { 400: s('same') }, '#cl-new');
    }
  });
}

function forgot() {
  const n = name.value.trim();
  if (!n) return fail(s('needName'), '#cl-user');
  attempt(async () => {
    try {
      await auth.askReset(n);
    } catch (e) {
      return explain(e, {}, '#cl-user');
    }
    logCode.value = next1.value = next2.value = '';
    go('reset', '#cl-code');
    toast(s('codeSent'));
  });
}

function reset() {
  const code = logCode.value.replace(/[\s-]/g, '');
  if (!/^[a-z0-9]{12}$/i.test(code)) return fail(s('twelve'), '#cl-code');
  if (!checkNew()) return;
  attempt(async () => {
    try {
      signedIn.value = await auth.reset(code, next1.value);
      go('resetDone');
    } catch (e) {
      explain(e, { 400: s('badReset') }, '#cl-code');
    }
  });
}

function passkey() {
  err.value = '';
  attempt(async () => {
    try {
      proceed(await auth.passkey(keep.value));
    } catch (e) {
      if (e instanceof ApiError) return explain(e, { 401: s('passkeyFailed'), 409: s('passkeyOff') });
      // The browser's own window was closed or found no passkey, which needs no scolding.
      if (!(e instanceof DOMException && e.name === 'NotAllowedError')) fail(s('passkeyFailed'));
    }
  });
}

function oidc() {
  if (!opts.value.oidc) return;
  step.value = 'leaving';
  location.assign(opts.value.oidc.url);
}

function back() {
  pass.value = '';
  byRecovery.value = false;
  go('pw');
}

function digitInput(i: number, e: Event) {
  const el = e.target as HTMLInputElement;
  const v = el.value.replace(/\D/g, '').slice(-1);
  digits.value[i] = v;
  el.value = v;
  if (v && i < 5) focus(`[data-d="${i + 1}"]`);
  if (digits.value.every((d) => d)) sendCode();
}
function digitKey(i: number, e: KeyboardEvent) {
  if (e.key === 'Backspace' && !digits.value[i] && i) {
    digits.value[i - 1] = '';
    focus(`[data-d="${i - 1}"]`);
  }
}
function digitPaste(e: ClipboardEvent) {
  const got = (e.clipboardData?.getData('text') || '').replace(/\D/g, '').slice(0, 6);
  if (!got) return;
  e.preventDefault();
  digits.value = Array.from({ length: 6 }, (_, k) => got[k] || '');
  if (got.length === 6) sendCode(); else focus(`[data-d="${got.length}"]`);
}
function toggleRecovery() {
  byRecovery.value = !byRecovery.value;
  err.value = '';
  focus(byRecovery.value ? '#cl-recovery' : '[data-d="0"]');
}
</script>

<template>
  <div class="cl" :class="{ over: overlay }">
    <Card class="cl-card glim-card" :title="title"><form ref="form" class="cl-form" novalidate @submit.prevent="submit">
      <div class="cl-brand">
        <span class="brand-mark"><G n="IconFolderOpen" /></span>
        <span><span class="cl-name">CannonCommander</span><span v-if="opts.server" class="muted">{{ s('on', { server: opts.server }) }}</span></span>
      </div>
      <div v-if="err" class="cl-err" role="alert"><G n="IconWarning" /><span>{{ err }}</span></div>
      <div v-else-if="note && step === 'pw'" class="cl-note" role="status"><G n="IconInfo" /><span>{{ note }}</span></div>

      <template v-if="step === 'pw'">
        <div class="cl-fields">
          <FieldRow :label="s('name')"><TextField id="cl-user" v-model="name" autocomplete="username" autocapitalize="off" spellcheck="false" width="grow" /></FieldRow>
          <FieldRow :label="s('password')"><PasswordField id="cl-pass" v-model="pass" autocomplete="current-password" width="grow" :text="eyeWords()" /></FieldRow>
          <FieldRow :label="s('stay')" :hint="s('stayTip')"><Toggle id="cl-keep" v-model="keep" /></FieldRow>
        </div>
        <div class="cl-btns">
          <Button class="glim-btn-key" :label="s('signIn')" label-key="IconSignIn" tone="accent" type="submit" :disabled="busy" />
          <Button v-if="opts.passkey" class="glim-btn-key" :label="s('passkey')" label-key="IconKey" data-cl="passkey" :disabled="busy" @click="passkey" />
          <Button v-if="opts.oidc" class="glim-btn-key" :label="s('oidc', { name: opts.oidc.label })" label-key="IconExternalLink" data-cl="oidc" @click="oidc" />
          <div class="cl-forgot"><Button tone="subtle" :label="s('forgot')" label-key="IconForgotPassword" data-cl="forgot" :disabled="busy" @click="forgot" /><InfoBubble :tip="s('forgotTip')" /></div>
        </div>
      </template>

      <template v-else-if="step === 'totp' || step === 'totpSetup'">
        <div v-if="step === 'totpSetup' && setup" class="cl-setup">
          <p class="cl-text">{{ s('setupText') }}</p>
          <!-- The server draws the QR code as SVG from the secret it just made. -->
          <div class="cl-qr" v-html="setup.qr"></div>
          <div class="cl-key"><span class="muted">{{ s('setupKey') }}</span><span class="mono">{{ setup.secret }}</span></div>
        </div>
        <div class="cl-totp">
          <div v-if="!byRecovery" class="cl-lab">{{ s('codeLabel') }} <InfoBubble v-if="step === 'totp'" :tip="s('codeTip')" /></div>
          <div v-if="!byRecovery" class="cl-digits">
            <TextField v-for="(d, i) in digits" :key="i" :model-value="d" class="glim-num" inputmode="numeric" maxlength="1" :autocomplete="i ? 'off' : 'one-time-code'" :aria-label="s('digit', { n: i + 1 })" :data-d="i" @input="digitInput(i, $event)" @keydown="digitKey(i, $event)" @paste="digitPaste" />
          </div>
          <FieldRow v-else :label="s('recovery')"><TextField id="cl-recovery" v-model="recovery" class="cl-code" placeholder="XXXX-XXXX" spellcheck="false" autocomplete="off" autocapitalize="characters" width="grow" mono /></FieldRow>
        </div>
        <div class="cl-btns">
          <Button class="glim-btn-key" :label="s('confirm')" label-key="IconConfirm" tone="accent" type="submit" :disabled="busy" />
          <Button v-if="step === 'totp'" tone="subtle" :label="byRecovery ? s('useApp') : s('useRecovery')" label-key="IconKey" data-cl="recovery" @click="toggleRecovery" />
          <Button tone="subtle" :label="s('back')" label-key="IconBack" data-cl="back" @click="back" />
        </div>
      </template>

      <template v-else-if="step === 'password' || step === 'reset'">
        <p v-if="step === 'password'" class="cl-text">{{ s('ownText') }}</p>
        <div class="cl-fields">
          <FieldRow v-if="step === 'reset'" :label="s('logCode')" :hint="s('logCodeTip')"><TextField id="cl-code" v-model="logCode" class="cl-code" placeholder="XXXX-XXXX-XXXX" spellcheck="false" autocomplete="one-time-code" autocapitalize="characters" width="grow" mono /></FieldRow>
          <FieldRow :label="s('newPassword')"><PasswordField id="cl-new" v-model="next1" autocomplete="new-password" :placeholder="s('minChars', { n: opts.minPassword })" width="grow" :text="eyeWords()" /></FieldRow>
          <FieldRow :label="s('repeat')"><PasswordField id="cl-new2" v-model="next2" autocomplete="new-password" width="grow" :text="eyeWords()" /></FieldRow>
        </div>
        <div class="cl-btns">
          <Button class="glim-btn-key" :label="s('setPassword')" label-key="IconKey" tone="accent" type="submit" :disabled="busy" />
          <Button tone="subtle" :label="s('back')" label-key="IconBack" data-cl="back" @click="back" />
        </div>
      </template>

      <template v-else-if="step === 'codes'">
        <p class="cl-text">{{ s('codesText') }}</p>
        <SubCard class="cl-codes mono glim-num"><span v-for="c in codes" :key="c">{{ c }}</span></SubCard>
        <div class="cl-btns"><Button class="glim-btn-key" :label="s('toFiles')" label-key="IconForward" tone="accent" type="submit" /></div>
      </template>

      <template v-else-if="step === 'resetDone'">
        <SubCard :title="s('goneToo')" class="cl-gone">
          <div v-for="k in ['goneSessions', 'goneTwofa', 'goneTokens']" :key="k" class="cl-li"><G n="IconCheck" /><span>{{ s(k) }}</span></div>
        </SubCard>
        <div class="cl-btns"><Button class="glim-btn-key" :label="s('toFiles')" label-key="IconForward" tone="accent" type="submit" /></div>
      </template>

      <div v-else class="cl-wait"><span class="cl-spin"></span><div>{{ s('leaving', { name: opts.oidc?.label ?? '' }) }}</div></div>
    </form></Card>
    <div v-if="opts.version" class="cl-ver muted glim-num">{{ s('version', { v: opts.version }) }}</div>
  </div>
</template>
