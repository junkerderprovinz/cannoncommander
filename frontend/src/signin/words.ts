// The words GlimStone's sign-in parts take, out of CannonCommander's language files, and the codes the kit answers
// with, each with a sentence of its own.
import {
  componentText, listOf, SIGN_IN_ERRORS, SIGN_IN_REDIRECTS, signInShortHint, signInTileTextEn, type Plural, type SignInErrorText,
  type SignInReauthText, type SignInRemoved, type SignInText, type SignInTileText,
} from '@glimstone/core';
import { ref } from 'vue';
import { forms, language, locale, plural, t } from '@/i18n';
import { copyWords, eyeWords } from '@/ui/kit';
import { signIn } from './kit';
import './texts';

// Codes of CannonCommander's own beside the kit's, and those a browser answers with.
export const ERROR_CODES = [...SIGN_IN_ERRORS, 'offline', 'cancelled', 'othersSignIn', 'passwordsOff', 'ownBlock', 'noSignIn'] as const;
export const ARRIVED_CODES = [...SIGN_IN_REDIRECTS, 'notAdmitted'] as const;

const s = (key: string) => t('signin.' + key);
const tile = (key: string) => s('tile.' + key);
// GlimStone fills in {count} where the language files, as everywhere in CannonCommander, say {n}.
const counted = (key: string) => Object.fromEntries(Object.entries(forms('signin.tile.' + key)).map(([k, v]) => [k, v.replace('{n}', '{count}')])) as Plural;

export const errorWords = (): SignInErrorText => Object.fromEntries(ERROR_CODES.map((code) => [code, s('errors.' + code)]));
export const arrivedWords = (): Record<string, string> => Object.fromEntries(ARRIVED_CODES.map((code) => [code, s('arrived.' + code)]));

export function cardText(): SignInText {
  const field = componentText(language()).passwordField;
  return {
    name: s('name'), password: s('password'), signIn: s('signIn'), signingIn: s('signingIn'), passkey: '', forgot: s('forgot'),
    code: s('codeLabel'), codeHint: s('codeTip'), codeInvalid: s('badCode'), back: s('back'),
    resetCode: s('logCode'), resetCodeTip: s('logCodeTip'), newPassword: s('newPassword'), confirmPassword: field.confirm,
    setPassword: s('setPassword'), saving: s('saving'), mismatch: field.mismatch, tooShort: '', shortHint: s('shortHint'),
    resetRejected: s('resetRejected'), failed: s('failed'), ...eyeWords(),
  };
}

// What the page after a reset says: the sessions that ended and whatever else the reset took away.
export function resetNote(removed: SignInRemoved): string {
  const count = (n: number, key: string) => plural(n, 'signin.tile.password.goes.' + key);
  const gone = [
    ...(removed.twoFactor ? [tile('password.goes.twoFactor')] : []),
    ...(removed.passkeys ? [count(removed.passkeys, 'passkeys')] : []),
    ...(removed.tokens ? [count(removed.tokens, 'tokens')] : []),
  ];
  return [s('resetDone'), ...(gone.length ? [t('signin.resetGone', { gone: listOf(locale(), gone) })] : []), ...(removed.services ? [s('resetServices')] : [])].join(' ');
}

// The length under which a new password gets its quiet hint, as the kit's offer says once it answered.
const suggested = ref(12);
let asked = false;

// The hint under a password CannonCommander sets, a share link's among them. A short one is taken all the same.
export function shortNote(password: string): string | undefined {
  if (!asked) {
    asked = true;
    void signIn.options().then((r) => { if (r.ok) suggested.value = r.value.minPassword; });
  }
  return signInShortHint(s('shortHint'), password, suggested.value);
}

// The sentence a saved password adds to its toast when the server found it short.
export const shortSaved = (answer: unknown) => ((answer as { short?: boolean } | undefined)?.short ? ' ' + s('longer') : '');

export function reauthText(): SignInReauthText {
  return {
    title: s('reauth.title'), text: s('reauth.text'), password: s('password'), confirm: s('reauth.confirm'),
    cancel: t('core.cancel'), close: t('core.close'), ...eyeWords(),
    wrong: s('errors.wrongPassword'), off: s('errors.confirmOff'), failed: s('failed'),
  };
}

export function tileText(): SignInTileText {
  const field = componentText(language()).passwordField;
  const days = (n: number) => plural(n, 'signin.tile.tokens.days');
  return {
    cancel: t('core.cancel'), close: t('core.close'), confirm: tile('confirm'), ...copyWords(), back: s('back'),
    save: t('config.save'), remove: t('config.remove'), ...eyeWords(), confirmPassword: field.confirm, mismatch: field.mismatch,
    failed: s('failed'),
    needsPassword: { title: tile('needsPassword.title'), reason: tile('needsPassword.reason') },
    notLocal: { title: tile('notLocal.title'), reason: tile('notLocal.reason') },
    password: {
      title: s('password'), hint: tile('password.hint'), openHint: tile('password.openHint'), viaService: tile('password.viaService'), state: s('password'),
      set: tile('password.set'), none: tile('password.none'), change: tile('password.change'), add: tile('password.add'),
      current: tile('password.current'), next: s('newPassword'), save: tile('password.save'), saved: tile('password.saved'),
      first: tile('password.first'), wrongCurrent: tile('password.wrongCurrent'), shortHint: s('shortHint'), shortSaved: s('longer'),
      remove: tile('password.remove'), removeTitle: tile('password.removeTitle'), removeText: tile('password.removeText'),
      goes: {
        twoFactor: tile('password.goes.twoFactor'), passkeys: counted('password.goes.passkeys'),
        services: tile('password.goes.services'), tokens: counted('password.goes.tokens'),
      },
      removed: tile('password.removed'),
    },
    askPassword: {
      title: tile('askPassword.title'), hint: tile('askPassword.hint'), label: tile('askPassword.label'),
      when: tile('askPassword.when'), on: tile('askPassword.on'), off: tile('askPassword.off'),
    },
    twoFactor: {
      title: tile('twoFactor.title'), hint: tile('twoFactor.hint'), state: tile('twoFactor.state'), on: tile('twoFactor.on'),
      off: tile('twoFactor.off'), setUp: tile('twoFactor.setUp'), codesLeft: tile('twoFactor.codes'),
      codesLeftValue: counted('twoFactor.codesLeft'), newCodes: tile('twoFactor.newCodes'), turnOff: tile('twoFactor.turnOff'),
      setupTitle: tile('twoFactor.setupTitle'), scan: tile('twoFactor.scan'), qr: tile('twoFactor.qr'), key: tile('twoFactor.key'),
      code: tile('twoFactor.code'), confirm: tile('twoFactor.turnOn'), sixDigits: tile('twoFactor.sixDigits'),
      codesTitle: tile('twoFactor.codes'), codesText: tile('twoFactor.codesText'), copyCodes: tile('twoFactor.copyCodes'),
      codesSaved: tile('twoFactor.codesSaved'), turnedOn: tile('twoFactor.turnedOn'), newCodesReady: tile('twoFactor.newCodesReady'),
      offTitle: tile('twoFactor.offTitle'), offText: tile('twoFactor.offText'), offConfirm: tile('twoFactor.turnOff'),
      turnedOff: tile('twoFactor.turnedOff'),
    },
    passkeys: {
      title: tile('passkeys.title'), hint: tile('passkeys.hint'), add: tile('passkeys.add'), none: tile('passkeys.none'),
      noneHint: tile('passkeys.noneHint'), created: tile('passkeys.created'), used: tile('passkeys.used'), never: tile('passkeys.never'),
      here: tile('passkeys.here'), other: tile('passkeys.other'), synced: tile('passkeys.synced'), rename: tile('passkeys.rename'),
      renameTitle: tile('passkeys.renameTitle'), removeText: tile('passkeys.removeText'), removed: tile('passkeys.removed'),
      added: tile('passkeys.added'), renamed: tile('passkeys.renamed'), addTitle: tile('passkeys.add'), name: s('name'),
      nameHint: tile('passkeys.nameHint'), fallbackName: tile('passkeys.fallbackName'), continue: tile('passkeys.continue'),
      unavailableTitle: tile('passkeys.unavailableTitle'), unavailable: tile('passkeys.unavailable'),
    },
    sessions: {
      title: tile('sessions.title'), hint: tile('sessions.hint'), current: tile('sessions.current'), end: s('logout'),
      endOthers: tile('sessions.endOthers'), ended: tile('sessions.ended'), othersEnded: tile('sessions.othersEnded'),
      from: tile('sessions.from'), seen: tile('sessions.seen'), on: tile('sessions.on'), browser: tile('sessions.browser'),
    },
    tokens: {
      title: tile('tokens.title'), hint: tile('tokens.hint'), create: tile('tokens.create'), none: tile('tokens.none'),
      noneHint: tile('tokens.noneHint'), scopes: { read: tile('tokens.read'), all: tile('tokens.all') },
      created: tile('tokens.created'), used: tile('tokens.used'), neverUsed: tile('tokens.neverUsed'), expires: tile('tokens.expires'),
      forever: tile('tokens.forever'), endsWhenClosed: tile('tokens.endsWhenClosed'), openHint: tile('tokens.openHint'), revoke: tile('tokens.revoke'), revokeText: tile('tokens.revokeText'), revoked: tile('tokens.revoked'),
      newTitle: tile('tokens.create'), name: s('name'), nameHint: tile('tokens.nameHint'), scope: tile('tokens.scope'),
      lifetime: tile('tokens.lifetime'), lifetimes: { '30': days(30), '90': days(90), '365': tile('tokens.year'), never: tile('tokens.never') },
      make: tile('tokens.make'), nameMissing: tile('tokens.nameMissing'), shownTitle: tile('tokens.shownTitle'),
      shownText: tile('tokens.shownText'), shownDone: tile('tokens.shownDone'),
    },
    services: {
      title: tile('services.title'), unnamed: tile('services.unnamed'), hint: tile('services.hint'), add: tile('services.add'), on: tile('twoFactor.on'),
      off: tile('twoFactor.off'), untested: tile('services.untested'), paused: tile('services.paused'), show: tile('services.show'),
      mustTest: tile('services.mustTest'), test: tile('services.test'), testing: tile('services.testing'), testOk: tile('services.testOk'),
      testFailed: tile('services.testFailed'), tested: tile('services.tested'), fromConfig: tile('services.fromConfig'),
      confirmAgain: tile('services.confirmAgain'), edit: tile('services.edit'), openBinds: tile('services.openBinds'),
      removeText: tile('services.removeText'), removed: tile('services.removed'), turnedOn: tile('services.turnedOn'),
      turnedOff: tile('services.turnedOff'), updated: tile('services.updated'), pick: tile('services.add'),
      pickHint: tile('services.pickHint'), group: tile('services.group'), own: tile('services.own'), ownSub: 'OpenID Connect',
      ownHint: tile('services.ownHint'), search: tile('services.search'), searchNone: tile('services.searchNone'),
      clear: tile('services.clear'), saved: tile('services.saved'), formHint: tile('services.formHint'),
      whoGetsIn: tile('services.whoGetsIn'), role: tile('services.role'), roleHint: tile('services.roleHint'), issuer: tile('services.issuer'), issuerHint: tile('services.issuerHint'),
      clientId: tile('services.clientId'), secret: tile('services.secret'), secretKept: tile('services.secretKept'),
      redirect: tile('services.redirect'), redirectHint: tile('services.redirectHint'), label: tile('services.label'),
      scopes: tile('services.scopes'), nameClaim: tile('services.nameClaim'), groupsClaim: tile('services.groupsClaim'), more: tile('services.more'),
      turnOn: tile('twoFactor.turnOn'), missing: tile('services.missing'), unreachable: tile('services.unreachable'),
    },
    // The Users tab manages the accounts, so the tile never draws their card.
    accounts: signInTileTextEn.accounts,
    roles: Object.fromEntries(['admin', 'user', 'readonly'].map((r) => [r, t('config.roles.' + r)])),
    via: {
      open: tile('via.open'), password: s('password'), passkey: tile('via.passkey'), service: '{name}',
      token: tile('via.token'), door: tile('via.door'),
    },
    errors: errorWords(),
    arrived: arrivedWords(),
  };
}
