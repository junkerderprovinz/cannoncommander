// The words GlimStone's sign-in parts take, out of CannonCommander's language files, and the codes the kit answers
// with, each with a sentence of its own.
import {
  componentText, listOf, SIGN_IN_ERRORS, SIGN_IN_REDIRECTS, signInTileTextEn, type Plural, type SignInErrorText,
  type SignInReauthText, type SignInRemoved, type SignInText, type SignInTileText,
} from '@glimstone/core';
import { forms, language, locale, num, plural, t } from '@/i18n';
import { copyWords, eyeWords } from '@/ui/kit';
import './texts';

// Codes of CannonCommander's own beside the kit's, and those a browser answers with.
export const ERROR_CODES = [...SIGN_IN_ERRORS, 'offline', 'cancelled', 'othersSignIn', 'fromConfig', 'passwordsOff', 'ownBlock', 'noSignIn'] as const;
export const ARRIVED_CODES = [...SIGN_IN_REDIRECTS, 'notAdmitted'] as const;

const s = (key: string) => t('signin.' + key);
const tile = (key: string) => s('tile.' + key);
const counted = (key: string) => forms('signin.tile.' + key) as Plural;

export const errorWords = (): SignInErrorText => Object.fromEntries(ERROR_CODES.map((code) => [code, s('errors.' + code)]));
export const arrivedWords = (): Record<string, string> => Object.fromEntries(ARRIVED_CODES.map((code) => [code, s('arrived.' + code)]));

export function cardText(minPassword: number): SignInText {
  const field = componentText(language()).passwordField;
  return {
    password: s('password'), signIn: s('signIn'), signingIn: s('signingIn'), passkey: '', forgot: s('forgot'),
    code: s('codeLabel'), codeHint: s('codeTip'), codeInvalid: s('badCode'), back: s('back'),
    resetCode: s('logCode'), resetCodeTip: s('logCodeTip'), newPassword: s('newPassword'), confirmPassword: field.confirm,
    setPassword: s('setPassword'), saving: s('saving'), mismatch: field.mismatch, tooShort: plural(minPassword, 'signin.short'),
    resetRejected: s('resetRejected'), failed: s('failed'), ...eyeWords(),
  };
}

// What the page after a reset says: the sessions that ended and whatever else the reset took away.
export function resetNote(removed: SignInRemoved): string {
  const count = (n: number, key: string) => plural(n, 'signin.tile.password.goes.' + key, { count: num(n) });
  const gone = [
    ...(removed.twoFactor ? [tile('password.goes.twoFactor')] : []),
    ...(removed.passkeys ? [count(removed.passkeys, 'passkeys')] : []),
    ...(removed.tokens ? [count(removed.tokens, 'tokens')] : []),
  ];
  return [s('resetDone'), ...(gone.length ? [t('signin.resetGone', { gone: listOf(locale(), gone) })] : []), ...(removed.services ? [s('resetServices')] : [])].join(' ');
}

export function reauthText(): SignInReauthText {
  return {
    title: s('reauth.title'), text: s('reauth.text'), password: s('password'), confirm: s('reauth.confirm'),
    cancel: t('core.cancel'), close: t('core.close'), ...eyeWords(),
    wrong: s('errors.wrongPassword'), off: s('errors.confirmOff'), failed: s('failed'),
  };
}

export function tileText(): SignInTileText {
  const field = componentText(language()).passwordField;
  const days = (n: number) => plural(n, 'signin.tile.tokens.days', { count: num(n) });
  return {
    cancel: t('core.cancel'), close: t('core.close'), confirm: tile('confirm'), ...copyWords(), back: s('back'),
    save: t('config.save'), remove: t('config.remove'), ...eyeWords(), confirmPassword: field.confirm, mismatch: field.mismatch,
    failed: s('failed'),
    needsPassword: { title: tile('needsPassword.title'), reason: tile('needsPassword.reason') },
    password: {
      title: s('password'), hint: tile('password.hint'), openHint: tile('password.openHint'), state: s('password'),
      set: tile('password.set'), none: tile('password.none'), change: tile('password.change'), add: tile('password.add'),
      current: tile('password.current'), next: s('newPassword'), save: tile('password.save'), saved: tile('password.saved'),
      first: tile('password.first'), wrongCurrent: tile('password.wrongCurrent'), tooShort: s('errors.passwordShort'),
      remove: tile('password.remove'), removeTitle: tile('password.removeTitle'), removeText: tile('password.removeText'),
      goes: {
        twoFactor: tile('password.goes.twoFactor'), passkeys: counted('password.goes.passkeys'),
        services: tile('password.goes.services'), tokens: counted('password.goes.tokens'),
      },
      removed: tile('password.removed'),
    },
    askPassword: {
      title: tile('askPassword.title'), hint: tile('askPassword.hint'), label: tile('askPassword.label'),
      on: tile('askPassword.on'), off: tile('askPassword.off'),
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
      forever: tile('tokens.forever'), revoke: tile('tokens.revoke'), revokeText: tile('tokens.revokeText'), revoked: tile('tokens.revoked'),
      newTitle: tile('tokens.create'), name: s('name'), nameHint: tile('tokens.nameHint'), scope: tile('tokens.scope'),
      lifetime: tile('tokens.lifetime'), lifetimes: { '30': days(30), '90': days(90), '365': tile('tokens.year'), never: tile('tokens.never') },
      make: tile('tokens.make'), nameMissing: tile('tokens.nameMissing'), shownTitle: tile('tokens.shownTitle'),
      shownText: tile('tokens.shownText'), shownDone: tile('tokens.shownDone'),
    },
    services: {
      title: tile('services.title'), hint: tile('services.hint'), add: tile('services.add'), on: tile('twoFactor.on'),
      off: tile('twoFactor.off'), untested: tile('services.untested'), paused: tile('services.paused'), show: tile('services.show'),
      mustTest: tile('services.mustTest'), test: tile('services.test'), testing: tile('services.testing'), testOk: tile('services.testOk'),
      testFailed: tile('services.testFailed'), tested: tile('services.tested'), edit: tile('services.edit'),
      removeText: tile('services.removeText'), removed: tile('services.removed'), turnedOn: tile('services.turnedOn'),
      turnedOff: tile('services.turnedOff'), updated: tile('services.updated'), pick: tile('services.add'),
      pickHint: tile('services.pickHint'), group: tile('services.group'), own: tile('services.own'), ownSub: 'OpenID Connect',
      ownHint: tile('services.ownHint'), search: tile('services.search'), searchNone: tile('services.searchNone'),
      clear: tile('services.clear'), saved: tile('services.saved'), formHint: tile('services.formHint'),
      whoGetsIn: tile('services.whoGetsIn'), issuer: tile('services.issuer'), issuerHint: tile('services.issuerHint'),
      clientId: tile('services.clientId'), secret: tile('services.secret'), secretKept: tile('services.secretKept'),
      redirect: tile('services.redirect'), redirectHint: tile('services.redirectHint'), label: tile('services.label'),
      scopes: tile('services.scopes'), nameClaim: tile('services.nameClaim'), groupsClaim: tile('services.groupsClaim'),
      turnOn: tile('twoFactor.turnOn'), missing: tile('services.missing'), unreachable: tile('services.unreachable'),
    },
    // The Users tab manages the accounts, so the tile never draws their card.
    accounts: signInTileTextEn.accounts,
    via: {
      open: tile('via.open'), password: s('password'), passkey: tile('via.passkey'), service: '{name}',
      token: tile('via.token'), door: tile('via.door'),
    },
    errors: errorWords(),
    arrived: arrivedWords(),
  };
}
