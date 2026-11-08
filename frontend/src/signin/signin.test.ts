import { createApp, nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { session } from '.';
import SignInPage from './SignInPage.vue';

type Answer = { status?: number; body?: unknown; headers?: Record<string, string> };
let script: Record<string, Answer | Answer[]> = {};
const sent: { url: string; body: unknown }[] = [];

beforeEach(() => {
  script = {};
  sent.length = 0;
  vi.stubGlobal('fetch', async (url: string, init: RequestInit = {}) => {
    const key = `${init.method ?? 'GET'} ${url.replace('/api/v1', '')}`;
    sent.push({ url: key, body: init.body ? JSON.parse(String(init.body)) : undefined });
    const entry = script[key];
    const a = (Array.isArray(entry) ? entry.shift() : entry) ?? { status: 404, body: { title: 'Not Found', status: 404 } };
    const status = a.status ?? 200;
    const type = status >= 400 ? 'application/problem+json' : 'application/json';
    return new Response(status === 204 ? null : JSON.stringify(a.body ?? {}), { status, headers: { 'Content-Type': type, ...a.headers } });
  });
  script['GET /auth/options'] = { body: { password: true, passkey: false, oidc: null, server: 'Bottich', version: '1.0', minPassword: 12 } };
});
afterEach(() => {
  vi.unstubAllGlobals();
  document.body.innerHTML = '';
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

async function mount(props: Record<string, unknown>) {
  const el = Object.assign(document.createElement('div'), { id: 'root' });
  document.body.append(el);
  createApp(SignInPage, props).mount(el);
  await settle();
}

describe('the sign in page', () => {
  it('shows the logo above the name and no version under the card', async () => {
    await mount({});
    expect(document.querySelector('.glim-signin-logo img')?.getAttribute('src')).toContain('cannoncommander');
    expect(document.querySelector('h1')?.textContent).toBe('CannonCommander');
    expect(document.body.textContent).not.toContain('Version');
    expect(document.body.textContent).not.toContain('Bottich');
  });

  it('starts in the name field and keeps Sign in off until a password is typed', async () => {
    await mount({});
    expect(document.activeElement).toBe($('#cl-user'));
    expect($('[data-signin="submit"]').disabled).toBe(true);
    type('#cl-password', 'x');
    await settle();
    expect($('[data-signin="submit"]').disabled).toBe(false);
  });

  it('shakes Sign in on a wrong password and selects the password', async () => {
    script['POST /auth/login'] = { status: 401, body: { title: 'Unauthorized', status: 401 } };
    await mount({});
    type('#cl-user', 'anna');
    type('#cl-password', 'falsch');
    await settle();
    submit();
    await settle();
    await frames();
    expect(error()).toContain('Name oder Passwort stimmt nicht');
    expect($('[data-signin="submit"]').classList).toContain('glim-shake');
    expect(document.activeElement).toBe($('#cl-password'));
    expect([$('#cl-password').selectionStart, $('#cl-password').selectionEnd]).toEqual([0, 6]);
  });

  it('asks for the code after the password and gives the app back when the session ran out', async () => {
    session.out = true;
    script['POST /auth/login'] = { body: { next: 'totp', challenge: 'c1' } };
    script['POST /auth/totp'] = [{ status: 400, body: { title: 'Bad Request', status: 400 } }, { body: { next: 'done', me: { name: 'anna', full: 'Anna', role: 'user' } } }];
    await mount({ overlay: true, was: 'anna' });
    expect($('#cl-user').value).toBe('anna');
    expect(document.activeElement).toBe($('#cl-password'));
    expect(document.querySelector('.glim-signin-note')?.textContent).toContain('Sitzung ist abgelaufen');

    type('#cl-password', 'anna-ihr-passwort');
    await settle();
    submit();
    await settle();
    expect(sent.find((s) => s.url === 'POST /auth/login')?.body).toEqual({ username: 'anna', password: 'anna-ihr-passwort', remember: true });
    expect(document.activeElement).toBe($('#cl-code'));

    type('#cl-code', '000000');
    await settle();
    submit();
    await settle();
    await frames();
    expect(error()).toContain('Code passt nicht');
    expect($('#cl-code').value).toBe('');
    expect($('[data-signin="submit"]').classList).toContain('glim-shake');

    type('#cl-code', '123456');
    await settle();
    submit();
    await settle();
    expect(sent.filter((s) => s.url === 'POST /auth/totp').map((s) => s.body)).toEqual([{ challenge: 'c1', code: '000000' }, { challenge: 'c1', code: '123456' }]);
    expect(session.out).toBe(false);
  });

  it('takes a recovery code in the code field', async () => {
    session.out = true;
    script['POST /auth/login'] = { body: { next: 'totp', challenge: 'c2' } };
    script['POST /auth/totp'] = { body: { next: 'done', me: { name: 'anna', full: 'Anna', role: 'user' } } };
    await mount({ overlay: true, was: 'anna' });
    type('#cl-password', 'anna-ihr-passwort');
    await settle();
    submit();
    await settle();
    type('#cl-code', 'k7qm-3xrp');
    await settle();
    submit();
    await settle();
    expect(sent.at(-1)).toEqual({ url: 'POST /auth/totp', body: { challenge: 'c2', code: 'k7qm-3xrp' } });
    expect(session.out).toBe(false);
  });

  it('says how long to wait after too many tries', async () => {
    script['POST /auth/login'] = { status: 429, body: { title: 'Too Many Requests', status: 429 }, headers: { 'Retry-After': '120' } };
    await mount({});
    type('#cl-user', 'anna');
    type('#cl-password', 'falsch');
    await settle();
    submit();
    await settle();
    expect(error()).toContain('warte 2 Minuten');
  });

  it('starts over when a step ran out', async () => {
    script['POST /auth/login'] = { body: { next: 'password', challenge: 'c3' } };
    script['POST /auth/password'] = { status: 410, body: { title: 'Gone', status: 410 } };
    await mount({});
    type('#cl-user', 'neu');
    type('#cl-password', 'aus-dem-log');
    await settle();
    submit();
    await settle();
    expect(document.querySelector('h2')?.textContent).toBe('Eigenes Passwort');
    type('#cl-new', 'ein-eigenes-passwort');
    type('#cl-new2', 'ein-anderes-passwort');
    submit();
    await settle();
    expect(error()).toContain('nicht gleich');
    type('#cl-new2', 'ein-eigenes-passwort');
    submit();
    await settle();
    expect(document.querySelector('#cl-user')).not.toBeNull();
    expect(document.querySelector('.glim-signin-note')?.textContent).toContain('zu lange gedauert');
  });

  it('wants the name before it asks for a reset code', async () => {
    script['POST /auth/reset-code'] = { status: 202 };
    await mount({});
    press('forgot');
    await settle();
    expect(error()).toContain('zuerst deinen Namen');
    type('#cl-user', 'anna');
    press('forgot');
    await settle();
    expect(sent.at(-1)).toEqual({ url: 'POST /auth/reset-code', body: { username: 'anna' } });
    expect(document.activeElement).toBe($('#cl-reset-code'));
  });

  it('shakes Set password on a wrong code from the log', async () => {
    script['POST /auth/reset-code'] = { status: 202 };
    script['POST /auth/reset'] = { status: 400, body: { title: 'Bad Request', status: 400 } };
    await mount({ was: 'anna' });
    press('forgot');
    await settle();
    type('#cl-reset-code', 'ABCD-EFGH-JKLM');
    type('#cl-new', 'ein-neues-langes-passwort');
    type('#cl-confirm', 'ein-neues-langes-passwort');
    await settle();
    submit();
    await settle();
    await frames();
    expect(sent.at(-1)).toEqual({ url: 'POST /auth/reset', body: { code: 'ABCDEFGHJKLM', password: 'ein-neues-langes-passwort' } });
    expect(error()).toContain('stimmt nicht oder ist abgelaufen');
    expect($('[data-signin="set-password"]').classList).toContain('glim-shake');
    expect(document.activeElement).toBe($('#cl-reset-code'));
  });

  it('says nothing when the passkey prompt is closed', async () => {
    script['GET /auth/options'] = { body: { password: true, passkey: true, oidc: null, server: '', version: '', minPassword: 12 } };
    script['POST /auth/passkey/options'] = { body: { challenge: 'p1', options: { publicKey: { challenge: 'AAAA', allowCredentials: [] } } } };
    vi.stubGlobal('PublicKeyCredential', class {});
    vi.stubGlobal('navigator', { ...navigator, credentials: { get: () => Promise.reject(new DOMException('closed', 'NotAllowedError')) } });
    await mount({});
    press('passkey');
    await settle();
    expect(document.querySelector('.glim-signin-error')).toBeNull();
  });

  it('offers no passkey in a browser that keeps none here', async () => {
    script['GET /auth/options'] = { body: { password: true, passkey: true, oidc: null, server: '', version: '', minPassword: 12 } };
    vi.stubGlobal('PublicKeyCredential', undefined);
    await mount({});
    expect(document.querySelector('[data-signin="passkey"]')).toBeNull();
  });
});
