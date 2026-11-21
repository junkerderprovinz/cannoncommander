// node tests/windows/signin.ts <server-url> <admin> <admin-password> [shots-dir]: signs in through the page against a
// built server on localhost, with and without two-factor authentication and with a passkey, then opens a
// share link with a password as a visitor, looks at a file, downloads one file and the folder as zip and uploads a
// file. The admin needs a source with a folder Urlaub holding Reiseplan.md and a subfolder. It changes the server's
// accounts and links as it goes, so it wants a fresh server.
import { createHmac } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..', '..', '..');
const { chromium, request } = createRequire(join(root, 'prototype', 'package.json'))('playwright') as typeof import('playwright');
const [base = 'http://localhost:8090', admin = 'jdp', adminPw = ''] = process.argv.slice(2);
const shots = resolve(process.argv[5] || join(root, 'testing', 'out', 'signin'));
mkdirSync(shots, { recursive: true });

let failed = 0;
const check = (what: string, ok: unknown, seen?: unknown) => {
  if (!ok) failed++;
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${what}${ok ? '' : `: ${JSON.stringify(seen)}`}`);
};

// RFC 6238 with the settings of the server: SHA-1, six digits, 30 seconds.
function totp(secret: string, at = Date.now()) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const c of secret.replace(/=+$/, '').toUpperCase()) bits += alphabet.indexOf(c).toString(2).padStart(5, '0');
  const key = Buffer.from(bits.match(/.{8}/g)!.map((b) => parseInt(b, 2)));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(at / 30000)));
  const h = createHmac('sha1', key).update(counter).digest();
  const n = (h.readUInt32BE(h[19]! & 15) & 0x7fffffff) % 1000000;
  return String(n).padStart(6, '0');
}

const api = await request.newContext({ baseURL: base });
// The sign-in takes changes only from the app's own pages, which a browser says in these headers.
const own = { Origin: base, 'Sec-Fetch-Site': 'same-origin' };
type Reply = { status(): number; headers(): Record<string, string>; json(): Promise<unknown> };
const answer = async (res: Reply) => ({ status: res.status(), body: res.headers()['content-type']?.includes('json') ? ((await res.json()) as any) : null });
const send = async (method: string, path: string, body?: unknown) => answer(await api.fetch('/api/v1' + path, { method, data: body, headers: own }));
const kit = async (method: string, path: string, body?: unknown) => answer(await api.fetch('/api/signin/v1/' + path, { method, data: body, headers: own }));

const signedIn = await kit('POST', 'login', { name: admin, password: adminPw });
check('the admin signs in through the sign-in', signedIn.status === 200 && signedIn.body?.visitor?.role === 'admin', signedIn.body);
const scope: string = (await send('GET', '/me')).body.scope;
const urlaub = scope.replace(/\/$/, '') + '/Urlaub';
check('the source has the folder Urlaub', (await send('GET', '/fs/list?path=' + encodeURIComponent(urlaub))).status === 200, urlaub);
const anna = await send('POST', '/admin/users', { name: 'anna', full: 'Anna', role: 'user', scope, password: 'anna-ihr-langes-passwort' });
check('anna is created', anna.status === 201, anna.body);
const setup = await kit('POST', 'me/totp', {});
const codes = await kit('POST', 'me/totp/confirm', { code: totp(setup.body.secret) });
check('two-factor authentication is on for the admin', codes.status === 200 && codes.body.codes.length > 0, codes.body);

const browser = await chromium.launch();
const errors: string[] = [];
async function newPage() {
  const context = await browser.newContext({ viewport: { width: 1280, height: 860 }, locale: 'de-AT', acceptDownloads: true });
  const page = await context.newPage();
  page.on('pageerror', (e: Error) => errors.push(String(e).slice(0, 300)));
  return { context, page };
}
const sessionCookie = async (context: { cookies(): Promise<{ name: string }[]> }) => (await context.cookies()).find((c) => c.name.endsWith('session'));
const hooks = (page: { locator(sel: string): { evaluateAll<T>(f: (els: Element[]) => T): Promise<T> } }, sel: string, key: string) =>
  page.locator(sel).evaluateAll((els) => els.map((x) => (x as HTMLElement).dataset[key]).join());

// Name and password, a wrong password first, then signing out again.
{
  const { context, page } = await newPage();
  await page.goto(base + '/');
  await page.waitForSelector('#cl-user');
  await page.screenshot({ path: join(shots, 'signin.png') });
  check('the name field has the focus', await page.evaluate(() => document.activeElement?.id === 'cl-user'));
  check('Sign in waits for a password', await page.isDisabled('[data-signin="submit"]'));
  check('the page offers Sign in and Forgot password? and nothing else', (await hooks(page, '#root button[data-signin]', 'signin')) === 'submit,forgot');
  await page.fill('#cl-user', 'anna');
  await page.fill('#cl-password', 'nicht-das-richtige');
  // The shake lasts a moment, so whether the button wore it is noted as it happens.
  await page.evaluate(() => {
    new MutationObserver(() => {
      if (document.querySelector('[data-signin="submit"]')?.classList.contains('glim-shake')) document.body.dataset.shook = 'yes';
    }).observe(document.body, { subtree: true, attributes: true, attributeFilter: ['class'] });
  });
  await page.click('[data-signin="submit"]');
  await page.waitForSelector('.glim-signin-error');
  check('a wrong password is explained', (await page.textContent('.glim-signin-error'))?.includes('Passwort stimmt nicht'));
  await page.waitForFunction(() => document.body.dataset.shook === 'yes', null, { timeout: 3000 }).catch(() => {});
  check('a wrong password shakes Sign in', await page.evaluate(() => document.body.dataset.shook === 'yes'));
  check('the password stays selected for the next try', await page.evaluate(() => {
    const field = document.activeElement as HTMLInputElement | null;
    return field?.id === 'cl-password' && field.selectionStart === 0 && field.selectionEnd === field.value.length;
  }));
  await page.screenshot({ path: join(shots, 'signin-wrong.png') });
  await page.fill('#cl-password', 'anna-ihr-langes-passwort');
  await page.click('[data-signin="submit"]');
  await page.waitForSelector('#app', { timeout: 30000 });
  await page.waitForTimeout(600);
  check('anna sees the file manager with a greeting', (await page.locator('.glim-toast').allInnerTexts()).some((t: string) => t.includes('Hallo')));
  check('the browser keeps a session of the sign-in', !!(await sessionCookie(context)));
  await page.screenshot({ path: join(shots, 'app-anna.png') });
  await page.locator('.glim-menubar-item', { hasText: 'Datei' }).click();
  await page.getByRole('menuitem', { name: /Abmelden/ }).click();
  await page.waitForSelector('#cl-user', { timeout: 30000 });
  check('signing out leads back to the sign in', (await page.textContent('.glim-signin-note'))?.includes('abgemeldet'));
  check('the sign out ended the session', (await page.evaluate(() => fetch('/api/v1/me').then((r) => r.status))) === 401);
  await page.screenshot({ path: join(shots, 'signed-out.png') });
  await context.close();
}

// Two-factor authentication, a passkey and an API token from the account tab.
{
  const { context, page } = await newPage();
  await page.goto(base + '/');
  await page.waitForSelector('#cl-user');
  await page.fill('#cl-user', admin);
  await page.fill('#cl-password', adminPw);
  await page.click('[data-signin="submit"]');
  await page.waitForSelector('#cl-code');
  await page.screenshot({ path: join(shots, 'signin-totp.png') });
  await page.fill('#cl-code', '000000');
  await page.keyboard.press('Enter');
  await page.waitForSelector('.glim-signin-error');
  check('a wrong code is explained', (await page.textContent('.glim-signin-error'))?.includes('Code passt nicht'));
  // The setup took the code of this time step, so the sign in takes the next one.
  await page.fill('#cl-code', totp(setup.body.secret, Date.now() + 30000));
  await page.keyboard.press('Enter');
  await page.waitForSelector('#app', { timeout: 30000 });
  await page.screenshot({ path: join(shots, 'app-admin.png') });

  const top = () => page.locator('[data-glim="Window"], .win').last();
  const settingsTab = async (name: string) => {
    await page.locator('.head [data-run="settings"]').click();
    await page.getByText(name, { exact: true }).first().click();
    await page.waitForTimeout(400);
  };
  const toasts = () => page.locator('.glim-toast').allInnerTexts();

  const cdp = await context.newCDPSession(page);
  await cdp.send('WebAuthn.enable');
  await cdp.send('WebAuthn.addVirtualAuthenticator', { options: { protocol: 'ctap2', transport: 'internal', hasResidentKey: true, hasUserVerification: true, isUserVerified: true } });
  await settingsTab('Konto & Sicherheit');
  check('the tile draws its cards without the accounts', (await hooks(page, '[data-signin-card]', 'signinCard')) === 'password,askPassword,twoFactor,passkeys,sessions,tokens,services');
  await page.click('[data-signin-tile="passkey-ask-add"]');
  await page.click('[data-signin-tile="passkey-add"]');
  await page.waitForTimeout(1500);
  check('a passkey is added from the tile', (await kit('GET', 'me/passkeys')).body?.passkeys?.length === 1 && (await toasts()).some((t: string) => t.includes('Passkey hinzugefügt')));
  await page.screenshot({ path: join(shots, 'passkey-added.png') });

  await page.click('[data-signin-tile="token-ask"]');
  await top().locator('input').first().fill('backup-skript');
  await page.click('[data-signin-tile="token-make"]');
  await page.waitForTimeout(800);
  check('a token from the tile shows once', (await top().textContent())?.includes('Dein neues Token'));
  await page.screenshot({ path: join(shots, 'token-shown.png') });
  check('the token is listed', (await kit('GET', 'me/tokens')).body?.tokens?.some((t: { name: string }) => t.name === 'backup-skript'));
  await top().getByRole('button', { name: 'Habe ich kopiert' }).click();

  // The browser offers the passkey in the password field, which a test runner cannot pick, so the page asks for it itself.
  await page.evaluate(() => fetch('/api/signin/v1/logout', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' }));
  await page.goto(base + '/');
  await page.waitForSelector('#cl-password');
  check('the password field lets the browser offer passkeys', (await page.getAttribute('#cl-password', 'autocomplete')) === 'current-password webauthn');
  check('the page has no passkey button', (await page.locator('[data-signin="passkey"]').count()) === 0);
  const viaPasskey = await page.evaluate(async () => {
    const b64 = (s: string) => Uint8Array.from(atob(s.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (s.length % 4)) % 4)), (c) => c.charCodeAt(0));
    const post = (path: string, body: unknown) => fetch('/api/signin/v1/' + path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    const { publicKey } = await (await post('login/passkey/options', {})).json();
    const cred = (await navigator.credentials.get({ publicKey: { ...publicKey, challenge: b64(publicKey.challenge), allowCredentials: [] } })) as PublicKeyCredential & { toJSON(): unknown };
    return (await post('login/passkey', cred.toJSON())).status;
  });
  check('the passkey signs in without name and code', viaPasskey === 200 && (await page.evaluate(() => fetch('/api/v1/me').then((r) => r.json()))).name === admin, viaPasskey);
  await page.goto(base + '/');
  await page.waitForSelector('#app', { timeout: 30000 });

  // Anna's own session and token, seen and ended by the admin, and then her account blocked.
  const annaApi = await request.newContext({ baseURL: base });
  await annaApi.post('/api/signin/v1/login', { data: { name: 'anna', password: 'anna-ihr-langes-passwort' }, headers: own });
  const annaToken = await annaApi.post('/api/signin/v1/me/tokens', { data: { name: 'fotos-sync', scopes: ['read'], days: 30 }, headers: own });
  check('anna makes a token', annaToken.status() === 200, annaToken.status());
  await settingsTab('Benutzer & Gruppen');
  const annaRow = () => page.locator('.glim-list-row, [data-uopen]', { hasText: 'Anna' }).first();
  await annaRow().click();
  await top().locator('[data-uaccess]').waitFor();
  await page.waitForTimeout(600);
  await top().locator('[data-uaccess]').click();
  await top().locator('[data-acc-token]').waitFor({ timeout: 10000 }).catch(() => {});
  check("the admin sees anna's session and token", (await top().locator('[data-acc-session]').count()) === 1 && (await top().locator('[data-acc-token]').count()) === 1);
  await page.screenshot({ path: join(shots, 'user-access.png') });
  await top().locator('[data-acc-token]').click();
  await top().getByRole('button', { name: 'Widerrufen' }).click();
  await page.waitForTimeout(800);
  const secret = ((await annaToken.json()) as { value: string }).value;
  check("the admin revokes anna's token", (await top().locator('[data-acc-token]').count()) === 0 && (await annaApi.get('/api/v1/me', { headers: { Authorization: 'Bearer ' + secret } })).status() === 401);
  await top().locator('[data-acc-all]').click();
  await top().getByRole('button', { name: 'Beenden', exact: true }).click();
  await page.waitForTimeout(800);
  check('ending all access signs anna out', (await top().locator('[data-acc-session]').count()) === 0 && (await annaApi.get('/api/v1/me')).status() === 401);
  await page.keyboard.press('Escape');
  await page.keyboard.press('Escape');
  await annaRow().click();
  await top().locator('#a-ublocked').click();
  await top().locator('[data-f]').last().click();
  await page.waitForTimeout(800);
  const again = await annaApi.post('/api/signin/v1/login', { data: { name: 'anna', password: 'anna-ihr-langes-passwort' }, headers: own });
  check('a blocked anna is marked and cannot sign in', (await annaRow().textContent())?.includes('Gesperrt') && again.status() === 403, again.status());
  await page.screenshot({ path: join(shots, 'user-blocked.png') });
  await annaApi.dispose();
  await context.close();
}

// A visitor opens a share link with a password, looks, downloads and uploads.
const share = await send('POST', '/shares', { paths: [urlaub], password: 'sonnenschein', uploads: true, message: 'Hallo Oma,\nhier sind die Fotos vom Urlaub.' });
check('the share is created', share.status === 201, share.body);
const once = await send('POST', '/shares', { paths: [urlaub + '/Reiseplan.md'], limit: 1 });
{
  const { context, page } = await newPage();
  await page.goto(`${base}/s/${share.body.slug}`);
  await page.waitForSelector('#sp-pass');
  check('the visitor page loads no part of the file manager', !(await page.evaluate(() => [...document.scripts].some((s) => /\/assets\/app-/.test(s.src)))));
  await page.screenshot({ path: join(shots, 'share-locked.png') });
  await page.fill('#sp-pass', 'regenbogen');
  await page.click('.sp-lock button[type=submit]');
  await page.waitForSelector('.sp-err');
  check('a wrong share password is explained', (await page.textContent('.sp-err'))?.includes('stimmt nicht'));
  await page.fill('#sp-pass', 'sonnenschein');
  await page.click('.sp-lock button[type=submit]');
  await page.waitForSelector('.sp-row');
  check('the message shows', (await page.textContent('.sp-msg'))?.includes('Hallo Oma'));
  check('the folder lists its files', (await page.locator('.sp-open .sp-nm').allInnerTexts()).join(',').includes('Reiseplan.md'));
  await page.screenshot({ path: join(shots, 'share.png'), fullPage: true });
  await page.click('.sp-open[data-name="Reiseplan.md"]');
  await page.waitForSelector('.sp-view pre');
  check('the text preview shows the file', (await page.textContent('.sp-view pre'))?.includes('Montag'));
  await page.screenshot({ path: join(shots, 'share-preview.png') });
  await page.keyboard.press('Escape');

  const [single] = await Promise.all([page.waitForEvent('download'), page.locator('.sp-row', { hasText: 'Reiseplan.md' }).locator('a').click()]);
  const text = readFileSync(await single.path(), 'utf8');
  check('a single file downloads', single.suggestedFilename() === 'Reiseplan.md' && text.includes('Montag'), single.suggestedFilename());
  const [zip] = await Promise.all([page.waitForEvent('download'), page.click('[data-sp="zip"]')]);
  const bytes = readFileSync(await zip.path());
  check('the folder downloads as zip', bytes.subarray(0, 2).toString() === 'PK' && zip.suggestedFilename().endsWith('.zip'), zip.suggestedFilename());

  const upload = join(shots, 'Gruesse.txt');
  const greeting = 'Liebe Grüße vom Besuch!\n'.repeat(2000);
  writeFileSync(upload, greeting);
  await page.setInputFiles('[data-sp="files"]', upload);
  await page.waitForSelector('.sp-send.done', { timeout: 30000 });
  await page.waitForSelector('.sp-open[data-name="Gruesse.txt"]', { timeout: 10000 });
  const there = (await send('GET', '/fs/list?path=' + encodeURIComponent(urlaub))).body.items.find((k: { name: string }) => k.name === 'Gruesse.txt');
  check('the upload arrives in the folder', there?.size === Buffer.byteLength(greeting), there);
  await page.screenshot({ path: join(shots, 'share-uploaded.png'), fullPage: true });

  await page.goto(`${base}/s/${once.body.slug}`);
  const [first] = await Promise.all([page.waitForEvent('download'), page.click('[data-sp="download"]')]);
  await first.path();
  await page.goto(`${base}/s/${once.body.slug}`);
  await page.waitForSelector('.sp-gone');
  check('a used up link explains itself', (await page.textContent('.sp-gone h1')) === 'Aufgebraucht');
  await page.screenshot({ path: join(shots, 'share-used-up.png') });
  await context.close();
}

check('no script errors', errors.length === 0, errors);
await browser.close();
await api.dispose();
console.log(failed ? `${failed} failed` : 'all passed');
process.exit(failed ? 1 : 0);
