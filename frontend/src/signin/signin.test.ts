import { createApp, nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { session } from '.';
import SignInPage from './SignInPage.vue';

const { left } = vi.hoisted(() => ({ left: { to: [] as string[], reloads: 0 } }));
vi.mock('./leave', () => ({ leaveFor: (url: string) => left.to.push(url), reload: () => { left.reloads++; } }));

type Answer = { status?: number; body?: unknown };
let script: Record<string, Answer | Answer[]> = {};
const sent: { url: string; body: unknown }[] = [];
const offer = (more: Record<string, unknown> = {}) => ({ open: false, owner: false, minPassword: 12, signedIn: false, passkeys: false, services: [], ...more });

beforeEach(() => {
  script = { 'GET options': { body: offer() } };
  sent.length = 0;
  vi.stubGlobal('fetch', async (url: string, init: RequestInit = {}) => {
    const key = `${init.method ?? 'GET'} ${url.replace('/api/signin/v1/', '')}`;
    sent.push({ url: key, body: init.body ? JSON.parse(String(init.body)) : undefined });
    const entry = script[key];
    const a = (Array.isArray(entry) ? entry.shift() : entry) ?? { status: 404, body: { error: 'notFound' } };
    return new Response(JSON.stringify(a.body ?? {}), { status: a.status ?? 200, headers: { 'Content-Type': 'application/json' } });
  });
  left.to = [];
  left.reloads = 0;
  history.replaceState(null, '', '/');
  session.out = false;
});
afterEach(() => {
  vi.unstubAllGlobals();
  document.body.innerHTML = '';
  sessionStorage.clear();
});

const settle = async () => {
  for (let i = 0; i < 6; i++) { await new Promise((r) => setTimeout(r, 0)); await nextTick(); }
};
// The shake starts two frames after the refusal.
const frames = () => new Promise((r) => setTimeout(r, 80));
const $ = (sel: string) => document.querySelector(sel) as HTMLInputElement;
function type(sel: string, value: string) {
  $(sel).value = value;
  $(sel).dispatchEvent(new Event('input'));
}
const submit = () => $('#root form').dispatchEvent(new Event('submit', { cancelable: true }));
const press = (hook: string) => (document.querySelector(`[data-signin="${hook}"]`) as HTMLButtonElement).click();
const error = () => document.querySelector('.glim-signin-error')?.textContent ?? '';
const buttons = () => [...document.querySelectorAll<HTMLElement>('#root button[data-signin]')].map((b) => b.dataset.signin);

async function mount(props: Record<string, unknown> = {}) {
  const el = Object.assign(document.createElement('div'), { id: 'root' });
  document.body.append(el);
  createApp(SignInPage, props).mount(el);
  await settle();
}

describe('the sign in page', () => {
  it('shows the logo above the name and no version under the card', async () => {
    await mount();
    expect(document.querySelector('.glim-signin-logo img')?.getAttribute('src')).toContain('cannoncommander');
    expect(document.querySelector('h1')?.textContent).toBe('CannonCommander');
    expect(document.body.textContent).not.toContain('Version');
  });

  it('starts in the name field and keeps Sign in off until a password is typed', async () => {
    await mount();
    expect(document.activeElement).toBe($('#cl-user'));
    expect($('[data-signin="submit"]').disabled).toBe(true);
    type('#cl-password', 'x');
    await settle();
    expect($('[data-signin="submit"]').disabled).toBe(false);
  });

  it('asks for no name where one account signs in', async () => {
    script['GET options'] = { body: offer({ owner: true }) };
    script['POST login'] = { body: { visitor: { name: 'jdp', role: 'admin', via: 'password' } } };
    await mount();
    expect($('#cl-user')).toBeNull();
    type('#cl-password', 'mein-passwort');
    await settle();
    submit();
    await settle();
    expect(sent.find((s) => s.url === 'POST login')?.body).toEqual({ password: 'mein-passwort' });
    expect(left.reloads).toBe(1);
  });

  it('shakes Sign in on a wrong password and selects the password', async () => {
    script['POST login'] = { status: 401, body: { error: 'wrongPassword' } };
    await mount();
    type('#cl-user', 'anna');
    type('#cl-password', 'falsch');
    await settle();
    submit();
    await settle();
    await frames();
    expect(sent.find((s) => s.url === 'POST login')?.body).toEqual({ name: 'anna', password: 'falsch' });
    expect(error()).toContain('Das Passwort stimmt nicht');
    expect($('[data-signin="submit"]').classList).toContain('glim-shake');
    expect(document.activeElement).toBe($('#cl-password'));
  });

  it('wants the name before it sends anything where several accounts sign in', async () => {
    await mount();
    type('#cl-password', 'passwort');
    await settle();
    submit();
    await settle();
    expect(error()).toContain('Gib Name und Passwort ein');
    expect(sent.some((s) => s.url === 'POST login')).toBe(false);
  });

  it('asks for the code after the password and gives the app back when the session ran out', async () => {
    session.out = true;
    script['POST login'] = { body: { step: 'code' } };
    script['POST login/totp'] = [{ status: 401, body: { error: 'wrongCode' } }, { body: { visitor: { name: 'anna', role: 'user', via: 'password' } } }];
    await mount({ overlay: true, was: 'anna' });
    expect($('#cl-user').value).toBe('anna');
    expect(document.activeElement).toBe($('#cl-password'));
    expect(document.querySelector('.glim-signin-note')?.textContent).toContain('Sitzung ist abgelaufen');

    type('#cl-password', 'anna-ihr-passwort');
    await settle();
    submit();
    await settle();
    expect(document.activeElement).toBe($('#cl-code'));

    type('#cl-code', '000000');
    await settle();
    submit();
    await settle();
    expect(error()).toContain('Code passt nicht');

    type('#cl-code', '123456');
    await settle();
    submit();
    await settle();
    expect(sent.filter((s) => s.url === 'POST login/totp').map((s) => s.body)).toEqual([{ code: '000000' }, { code: '123456' }]);
    expect(session.out).toBe(false);
    expect(left.reloads).toBe(0);
  });

  it('says to wait after too many tries', async () => {
    script['POST login'] = { status: 429, body: { error: 'throttled', retryAfter: 60 } };
    await mount();
    type('#cl-user', 'anna');
    type('#cl-password', 'falsch');
    await settle();
    submit();
    await settle();
    expect(error()).toBe('Zu viele Versuche. Versuch es in 60 Sekunden noch einmal.');
  });

  it('asks for the reset code, then for the name in the reset form, and says afterwards what the reset took away', async () => {
    script['POST reset-code'] = { body: {} };
    script['POST reset'] = { body: { visitor: { name: 'anna', role: 'admin', via: 'password' }, removed: { tokens: 2, passkeys: 1, twoFactor: true, services: 0 } } };
    await mount();
    press('forgot');
    await settle();
    expect(sent.at(-1)).toEqual({ url: 'POST reset-code', body: {} });
    expect(document.activeElement).toBe($('#cl-reset-code'));
    type('#cl-reset-code', 'ABCD-EFGH-JKLM');
    type('#cl-new', 'ein-neues-langes-passwort');
    type('#cl-confirm', 'ein-neues-langes-passwort');
    await settle();
    expect($('[data-signin="set-password"]').disabled).toBe(true);
    type('#cl-reset-name', 'anna');
    await settle();
    submit();
    await settle();
    expect(sent.at(-1)).toEqual({ url: 'POST reset', body: { code: 'ABCD-EFGH-JKLM', password: 'ein-neues-langes-passwort', name: 'anna' } });
    expect(sessionStorage.getItem('cannoncommander-signin-note')).toBe(
      'Dein neues Passwort ist gesetzt, und alle anderen Sitzungen sind beendet. Ebenfalls weg: Zwei-Faktor-Authentifizierung, 1 Passkey und 2 API-Tokens.');
    expect(left.reloads).toBe(1);
  });

  it('shakes Set password on a wrong code from the log', async () => {
    script['POST reset-code'] = { body: {} };
    script['POST reset'] = { status: 401, body: { error: 'resetRefused' } };
    await mount();
    press('forgot');
    await settle();
    type('#cl-reset-code', 'ABCD-EFGH-JKLM');
    type('#cl-reset-name', 'anna');
    type('#cl-new', 'ein-neues-langes-passwort');
    type('#cl-confirm', 'ein-neues-langes-passwort');
    await settle();
    submit();
    await settle();
    await frames();
    expect(error()).toContain('Ist er abgelaufen, steht jetzt ein neuer im Log');
    expect($('[data-signin="set-password"]').classList).toContain('glim-shake');
  });

  it('offers two buttons at most: Sign in and the sign-in service, with Forgot password as a plain link', async () => {
    script['GET options'] = { body: offer({ passkeys: true, services: [{ id: 's1', provider: 'authentik', name: 'authentik' }, { id: 's2', provider: 'custom', name: 'Zweiter' }] }) };
    await mount();
    expect(buttons()).toEqual(['submit', 'service', 'forgot']);
    expect(document.querySelector('[data-signin="service"]')?.textContent).toContain('authentik');
    expect(document.querySelector('[data-signin="service"] svg')).not.toBeNull();
    expect(document.querySelector('[data-signin="forgot"]')?.classList).toContain('glim-signin-forgot');
    press('service');
    expect(left.to).toEqual(['/signin/oidc/s1?return=%2F']);
  });

  it('starts the service of config.yaml where it always started', async () => {
    script['GET options'] = { body: offer({ services: [{ id: 'config', provider: 'custom', name: 'OIDC' }] }) };
    await mount();
    press('service');
    expect(left.to).toEqual(['/api/auth/oidc/login?redirect=%2F']);
  });

  it('lets the browser offer passkeys in the password field, without a button', async () => {
    script['GET options'] = { body: offer({ passkeys: true }) };
    await mount();
    expect($('#cl-password').getAttribute('autocomplete')).toBe('current-password webauthn');
    expect(document.querySelector('[data-signin="passkey"]')).toBeNull();
  });

  it('says what a sign-in service sent back, once', async () => {
    history.replaceState(null, '', '/?signin=notAdmitted');
    await mount();
    expect(error()).toContain('keine deiner Gruppen dort darf CannonCommander benutzen');
    expect(location.search).toBe('');
  });
});
