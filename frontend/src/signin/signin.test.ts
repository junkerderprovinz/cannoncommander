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
const $ = (sel: string) => document.querySelector(sel) as HTMLInputElement;
function type(sel: string, value: string) {
  $(sel).value = value;
  $(sel).dispatchEvent(new Event('input'));
}
const submit = () => $('#root form').dispatchEvent(new Event('submit', { cancelable: true }));

async function mount(props: Record<string, unknown>) {
  const el = Object.assign(document.createElement('div'), { id: 'root' });
  document.body.append(el);
  createApp(SignInPage, props).mount(el);
  await settle();
}

describe('the sign in page', () => {
  it('asks for the code after the password and gives the app back when the session ran out', async () => {
    session.out = true;
    script['POST /auth/login'] = { body: { next: 'totp', challenge: 'c1' } };
    script['POST /auth/totp'] = [{ status: 400, body: { title: 'Bad Request', status: 400 } }, { body: { next: 'done', me: { name: 'anna', full: 'Anna', role: 'user' } } }];
    await mount({ overlay: true, was: 'anna' });
    expect($('#cl-user').value).toBe('anna');
    expect(document.querySelector('.cl-note')?.textContent).toContain('Sitzung ist abgelaufen');

    type('#cl-pass', 'anna-ihr-passwort');
    submit();
    await settle();
    expect(sent.find((s) => s.url === 'POST /auth/login')?.body).toEqual({ username: 'anna', password: 'anna-ihr-passwort', remember: true });
    expect(document.querySelectorAll('[data-d]')).toHaveLength(6);

    '000000'.split('').forEach((d, i) => type(`[data-d="${i}"]`, d));
    await settle();
    expect(document.querySelector('.cl-err')?.textContent).toContain('Code passt nicht');
    expect($('[data-d="0"]').value).toBe('');

    '123456'.split('').forEach((d, i) => type(`[data-d="${i}"]`, d));
    await settle();
    expect(sent.filter((s) => s.url === 'POST /auth/totp').map((s) => s.body)).toEqual([{ challenge: 'c1', code: '000000' }, { challenge: 'c1', code: '123456' }]);
    expect(session.out).toBe(false);
  });

  it('takes a recovery code instead of the digits', async () => {
    session.out = true;
    script['POST /auth/login'] = { body: { next: 'totp', challenge: 'c2' } };
    script['POST /auth/totp'] = { body: { next: 'done', me: { name: 'anna', full: 'Anna', role: 'user' } } };
    await mount({ overlay: true, was: 'anna' });
    type('#cl-pass', 'anna-ihr-passwort');
    submit();
    await settle();
    (document.querySelector('[data-cl="recovery"]') as HTMLButtonElement).click();
    await settle();
    type('#cl-recovery', 'k7qm-3xrp');
    submit();
    await settle();
    expect(sent.at(-1)).toEqual({ url: 'POST /auth/totp', body: { challenge: 'c2', code: 'k7qm-3xrp' } });
    expect(session.out).toBe(false);
  });

  it('says how long to wait after too many tries', async () => {
    script['POST /auth/login'] = { status: 429, body: { title: 'Too Many Requests', status: 429 }, headers: { 'Retry-After': '120' } };
    await mount({});
    type('#cl-user', 'anna');
    type('#cl-pass', 'falsch');
    submit();
    await settle();
    expect(document.querySelector('.cl-err')?.textContent).toContain('warte 2 Minuten');
  });

  it('starts over when a step ran out', async () => {
    script['POST /auth/login'] = { body: { next: 'password', challenge: 'c3' } };
    script['POST /auth/password'] = { status: 410, body: { title: 'Gone', status: 410 } };
    await mount({});
    type('#cl-user', 'neu');
    type('#cl-pass', 'aus-dem-log');
    submit();
    await settle();
    type('#cl-new', 'ein-eigenes-passwort');
    type('#cl-new2', 'ein-anderes-passwort');
    submit();
    await settle();
    expect(document.querySelector('.cl-err')?.textContent).toContain('nicht gleich');
    type('#cl-new2', 'ein-eigenes-passwort');
    submit();
    await settle();
    expect(document.querySelector('#cl-user')).not.toBeNull();
    expect(document.querySelector('.cl-err')?.textContent).toContain('zu lange gedauert');
  });

  it('wants the name before it asks for a reset code', async () => {
    script['POST /auth/reset-code'] = { status: 202 };
    await mount({});
    (document.querySelector('[data-cl="forgot"]') as HTMLButtonElement).click();
    await settle();
    expect(document.querySelector('.cl-err')?.textContent).toContain('zuerst deinen Namen');
    type('#cl-user', 'anna');
    (document.querySelector('[data-cl="forgot"]') as HTMLButtonElement).click();
    await settle();
    expect(sent.at(-1)).toEqual({ url: 'POST /auth/reset-code', body: { username: 'anna' } });
    expect(document.querySelector('#cl-code')).not.toBeNull();
  });
});
