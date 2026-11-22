import { SIGN_IN_ERRORS, SIGN_IN_REDIRECTS, signInErrorText } from '@glimstone/core';
import { afterEach, describe, expect, it } from 'vitest';
import { ApiError } from '@/api/client';
import { addMessages, setLanguage } from '@/i18n';
import configDe from '@/features/config/de.json';
import configEn from '@/features/config/en.json';
import { ARRIVED_CODES, arrivedWords, cardText, ERROR_CODES, errorWords, reauthText, resetNote, tileText } from './words';

addMessages('config', { de: configDe, en: configEn });
afterEach(() => setLanguage('de'));

// Every string inside the words, with where it stands.
function strings(value: unknown, at = ''): [string, string][] {
  if (typeof value === 'string') return [[at, value]];
  if (value && typeof value === 'object') return Object.entries(value).flatMap(([k, v]) => strings(v, at ? `${at}.${k}` : k));
  return [];
}
const unsaid = (words: unknown) => strings(words).filter(([, s]) => /^(signin|config|core|ui)\.[\w.]+$/.test(s)).map(([at]) => at);

describe('the sign-in words', () => {
  for (const lang of ['de', 'en']) {
    it(`give every code of the kit and of CannonCommander a sentence of its own in ${lang}`, async () => {
      await setLanguage(lang);
      const errors = errorWords() as Record<string, string>, arrived = arrivedWords();
      for (const code of [...SIGN_IN_ERRORS, 'offline', 'cancelled', 'othersSignIn', 'fromConfig', 'passwordsOff', 'ownBlock', 'noSignIn']) {
        expect(errors[code], code).toMatch(/\w{3}/);
        expect(errors[code], code).not.toContain('signin.');
      }
      for (const code of [...SIGN_IN_REDIRECTS, 'notAdmitted']) {
        expect(arrived[code], code).toMatch(/\w{3}/);
        expect(arrived[code], code).not.toContain('signin.');
      }
    });

    it(`leave no word of the card, the tile or the password question unsaid in ${lang}`, async () => {
      await setLanguage(lang);
      expect(unsaid(cardText())).toEqual([]);
      expect(unsaid(tileText())).toEqual([]);
      expect(unsaid(reauthText())).toEqual([]);
    });
  }

  it('list each code once', () => {
    expect(new Set(ERROR_CODES).size).toBe(ERROR_CODES.length);
    expect(new Set(ARRIVED_CODES).size).toBe(ARRIVED_CODES.length);
  });

  it('fill in the numbers the kit sends with a code', () => {
    const words = { errors: errorWords(), failed: 'x' };
    expect(signInErrorText(words, { error: 'passwordShort', min: 14 })).toBe('Das neue Passwort braucht mindestens 14 Zeichen.');
    expect(signInErrorText(words, { error: 'scopeUnknown', scope: 'write' })).toBe('Den Scope write kennt CannonCommander nicht.');
  });

  it('say the token lifetimes and the counts in the language\'s own forms', () => {
    const t = tileText();
    expect(t.tokens.lifetimes).toEqual({ '30': '30 Tage', '90': '90 Tage', '365': '1 Jahr', never: 'Immer' });
    expect(t.password.goes.passkeys).toEqual({ one: '{count} Passkey', other: '{count} Passkeys' });
    expect(t.tokens.scopes).toEqual({ read: 'Lesen', all: 'Lesen und schreiben' });
  });

  it('take the second password field and its mismatch from GlimStone, as every PasswordField says them', () => {
    expect(cardText()).toMatchObject({ confirmPassword: 'Passwort bestätigen', mismatch: 'Die Passwörter stimmen nicht überein' });
  });

  it('name what a reset took away, and leave out what it did not', () => {
    expect(resetNote({ tokens: 0, passkeys: 0, twoFactor: false, services: 0 })).toBe('Dein neues Passwort ist gesetzt, und alle anderen Sitzungen sind beendet.');
    expect(resetNote({ tokens: 1, passkeys: 3, twoFactor: false, services: 2 })).toBe(
      'Dein neues Passwort ist gesetzt, und alle anderen Sitzungen sind beendet. Ebenfalls weg: 3 Passkeys und 1 API-Token. '
      + 'Anmeldedienste bleiben pausiert, bis eine Testanmeldung wieder klappt.');
  });

  it('say a code of the sign-in in the same words when CannonCommander\'s own API passes it on', () => {
    const refused = new ApiError(400, { type: '', title: 'Bad Request', status: 400, code: 'ownBlock' });
    expect(refused.message).toBe('Dein eigenes Konto kannst du nicht sperren.');
  });
});
