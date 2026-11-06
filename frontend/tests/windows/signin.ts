// node tests/windows/signin.ts <server-url> <admin> <admin-password> [shots-dir]: signs in through the page against a
// built server with passkeys on for localhost, without and with two factor login and with a passkey, then opens a
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
const send = async (method: string, path: string, body?: unknown) => {
  const res = await api.fetch('/api/v1' + path, { method, data: body });
  return { status: res.status(), body: res.headers()['content-type']?.includes('json') ? await res.json() : null };
};

const signedIn = await send('POST', '/auth/login', { username: admin, password: adminPw });
check('the admin signs in through the API', signedIn.body?.next === 'done', signedIn.body);
const scope: string = signedIn.body.me.scope;
const urlaub = scope.replace(/\/$/, '') + '/Urlaub';
check('the source has the folder Urlaub', (await send('GET', '/fs/list?path=' + encodeURIComponent(urlaub))).status === 200, urlaub);
const anna = await send('POST', '/admin/users', { name: 'anna', full: 'Anna', role: 'user', scope, password: 'anna-ihr-langes-passwort' });
check('anna is created', anna.status === 201, anna.body);
const setup = await send('POST', '/me/totp', {});
const codes = await send('POST', '/me/totp', { code: totp(setup.body.secret) });
check('two factor login is on for the admin', codes.status === 200 && codes.body.codes.length === 8, codes.body);

const browser = await chromium.launch();
const errors: string[] = [];
async function newPage() {
  const context = await browser.newContext({ viewport: { width: 1280, height: 860 }, locale: 'de-AT', acceptDownloads: true });
  const page = await context.newPage();
  page.on('pageerror', (e: Error) => errors.push(String(e).slice(0, 300)));
  return { context, page };
}
const sessionCookie = async (context: { cookies(): Promise<{ name: string; expires: number }[]> }) => (await context.cookies()).find((c) => c.name === 'cannoncommander_session');

// Without two factor login and without staying signed in: the cookie ends with the browser.
{
  const { context, page } = await newPage();
  await page.goto(base + '/');
  await page.waitForSelector('#cl-user');
  await page.screenshot({ path: join(shots, 'signin.png') });
  await page.fill('#cl-user', 'anna');
  await page.fill('#cl-pass', 'nicht-das-richtige');
  await page.click('#cl-keep');
  await page.click('.cl-card button[type=submit]');
  await page.waitForSelector('.cl-err');
  check('a wrong password is explained', (await page.textContent('.cl-err'))?.includes('Name oder Passwort stimmt nicht'));
  await page.screenshot({ path: join(shots, 'signin-wrong.png') });
  await page.fill('#cl-pass', 'anna-ihr-langes-passwort');
  await page.click('.cl-card button[type=submit]');
  await page.waitForSelector('#app', { timeout: 30000 });
  await page.waitForTimeout(600);
  check('anna sees the file manager with a greeting', (await page.locator('.glim-toast').allInnerTexts()).some((t: string) => t.includes('Hallo Anna')));
  const cookie = await sessionCookie(context);
  check('the session ends with the browser', cookie && cookie.expires === -1, cookie);
  await page.screenshot({ path: join(shots, 'app-anna.png') });
  await page.locator('.glim-menubar-item', { hasText: 'Datei' }).click();
  await page.getByRole('menuitem', { name: /Abmelden/ }).click();
  await page.waitForSelector('#cl-user', { timeout: 30000 });
  check('signing out leads back to the sign in', (await page.textContent('.cl-note'))?.includes('abgemeldet'));
  check('the sign out ended the session', (await page.evaluate(() => fetch('/api/v1/me').then((r) => r.status))) === 401);
  await page.screenshot({ path: join(shots, 'signed-out.png') });
  await context.close();
}

// With two factor login and staying signed in, then a passkey for the next time.
{
  const { context, page } = await newPage();
  await page.goto(base + '/');
  await page.waitForSelector('#cl-user');
  await page.fill('#cl-user', admin);
  await page.fill('#cl-pass', adminPw);
  await page.click('.cl-card button[type=submit]');
  await page.waitForSelector('[data-d="0"]');
  await page.screenshot({ path: join(shots, 'signin-totp.png') });
  await page.locator('[data-d="0"]').pressSequentially('000000');
  await page.waitForSelector('.cl-err');
  check('a wrong code is explained', (await page.textContent('.cl-err'))?.includes('Code passt nicht'));
  // The setup took the code of this time step, so the sign in takes the next one.
  await page.locator('[data-d="0"]').pressSequentially(totp(setup.body.secret, Date.now() + 30000));
  await page.waitForSelector('#app', { timeout: 30000 });
  const cookie = await sessionCookie(context);
  const days = cookie ? (cookie.expires * 1000 - Date.now()) / 86400000 : 0;
  check('staying signed in keeps the cookie for 30 days', days > 29.9 && days < 30.1, days);
  await page.screenshot({ path: join(shots, 'app-admin.png') });

  const cdp = await context.newCDPSession(page);
  await cdp.send('WebAuthn.enable');
  await cdp.send('WebAuthn.addVirtualAuthenticator', { options: { protocol: 'ctap2', transport: 'internal', hasResidentKey: true, hasUserVerification: true, isUserVerified: true } });
  const asked = await page.evaluate(async (pw) => {
    const post = (body: unknown) => fetch('/api/v1/me/passkeys', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).then((r) => r.status);
    return [await post({ name: 'Testgerät' }), await post({ name: 'Testgerät', password: pw + '-falsch' })];
  }, adminPw);
  check('a passkey wants the current password, and the right one', asked[0] === 400 && asked[1] === 403, asked);
  const added = await page.evaluate(async (pw) => {
    const b64 = (s: string) => Uint8Array.from(atob(s.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (s.length % 4)) % 4)), (c) => c.charCodeAt(0));
    const post = (body: unknown) => fetch('/api/v1/me/passkeys', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    const { options } = await (await post({ name: 'Testgerät', password: pw })).json();
    const pk = options.publicKey;
    const credential = await navigator.credentials.create({ publicKey: { ...pk, challenge: b64(pk.challenge), user: { ...pk.user, id: b64(pk.user.id) }, excludeCredentials: [] } });
    return (await post({ name: 'Testgerät', credential: (credential as PublicKeyCredential & { toJSON(): unknown }).toJSON() })).status;
  }, adminPw);
  check('a passkey is registered', added === 201, added);
  await page.evaluate(() => fetch('/api/v1/auth/logout', { method: 'POST' }));
  await page.goto(base + '/');
  await page.waitForSelector('[data-cl="passkey"]');
  await page.click('[data-cl="passkey"]');
  await page.waitForSelector('#app', { timeout: 30000 });
  check('the passkey signs in without name and code', (await page.evaluate(() => fetch('/api/v1/me').then((r) => r.json()))).name === admin);

  // A token from the security tab asks for the password and says so when it is wrong.
  const top = () => page.locator('.win').last();
  const settingsTab = async (name: string) => {
    await page.locator('[aria-label="Einstellungen"]').first().click();
    await page.getByText(name, { exact: true }).first().click();
    await page.waitForTimeout(400);
  };
  await settingsTab('Konto & Sicherheit');
  await page.click('[data-tadd]');
  await top().locator('#a-tkname').fill('backup-skript');
  await page.screenshot({ path: join(shots, 'token-password.png') });
  await top().getByRole('button', { name: 'Erstellen' }).click();
  await page.waitForTimeout(500);
  check('a token without the password is held back', (await page.locator('.glim-toast').allInnerTexts()).some((t: string) => t.includes('aktuelle Passwort fehlt')));
  await top().locator('#a-tkpw').fill(adminPw + '-falsch');
  await top().getByRole('button', { name: 'Erstellen' }).click();
  await page.waitForTimeout(800);
  check('a wrong password is explained in the token window', (await page.locator('.glim-toast').allInnerTexts()).some((t: string) => t.includes('stimmt nicht')) && (await top().locator('#a-tkpw').count()) === 1);
  await top().locator('#a-tkpw').fill(adminPw);
  await top().getByRole('button', { name: 'Erstellen' }).click();
  await top().locator('.glim-copy-field, input[readonly]').first().waitFor({ timeout: 10000 }).catch(() => {});
  check('the right password makes the token', (await send('GET', '/me/tokens')).body?.items?.some((t: { name: string }) => t.name === 'backup-skript'));
  await top().getByRole('button', { name: 'Fertig' }).click();

  // Anna's own session and token, seen and ended by the admin.
  const annaApi = await request.newContext({ baseURL: base });
  await annaApi.post('/api/v1/auth/login', { data: { username: 'anna', password: 'anna-ihr-langes-passwort' } });
  const annaToken = await annaApi.post('/api/v1/me/tokens', { data: { name: 'fotos-sync', rights: 'r', password: 'anna-ihr-langes-passwort' } });
  check('anna makes a token with her password', annaToken.status() === 201, annaToken.status());
  await settingsTab('Benutzer & Gruppen');
  await page.locator('.glim-list-row, [data-uopen]', { hasText: 'Anna' }).first().click();
  await top().locator('[data-uaccess]').waitFor();
  await page.waitForTimeout(600);
  await page.screenshot({ path: join(shots, 'user-access-row.png') });
  await top().locator('[data-uaccess]').click();
  await top().locator('[data-acc-token]').waitFor({ timeout: 10000 }).catch(() => {});
  check("the admin sees anna's session and token", (await top().locator('[data-acc-session]').count()) === 1 && (await top().locator('[data-acc-token]').count()) === 1);
  await page.screenshot({ path: join(shots, 'user-access.png') });
  await top().locator('[data-acc-token]').click();
  await top().getByRole('button', { name: 'Widerrufen' }).click();
  await page.waitForTimeout(800);
  check("the admin revokes anna's token", (await top().locator('[data-acc-token]').count()) === 0 && (await annaApi.get('/api/v1/me', { headers: { Authorization: 'Bearer ' + (await annaToken.json()).secret } })).status() === 401);
  await top().locator('[data-acc-all]').click();
  await top().getByRole('button', { name: 'Beenden', exact: true }).click();
  await page.waitForTimeout(800);
  check('ending all access signs anna out', (await top().locator('[data-acc-session]').count()) === 0 && (await annaApi.get('/api/v1/me')).status() === 401);
  await page.screenshot({ path: join(shots, 'user-access-ended.png') });
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
