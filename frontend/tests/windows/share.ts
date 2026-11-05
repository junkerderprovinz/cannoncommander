// node tests/windows/share.ts [app-url] [shots-dir]: shares files and a link to the app as a phone does, through the
// service worker in the app running with `VITE_E2E=1 vite --mode fixture`, and checks that the app asks where they
// go and uploads them there. It changes the fixture's tree, so it wants a fixture server of its own, started fresh.
import { mkdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..', '..', '..');
const { chromium } = createRequire(join(root, 'prototype', 'package.json'))('playwright') as typeof import('playwright');
const app = process.argv[2] || 'http://localhost:5191/';
const shots = resolve(process.argv[3] || join(root, 'testing', 'out', 'windows'));
mkdirSync(shots, { recursive: true });

const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1600, height: 950 }, locale: 'de-AT', serviceWorkers: 'allow' });
const page = await context.newPage();
const errors: string[] = [];
page.on('pageerror', (e: Error) => errors.push(String(e).slice(0, 300)));
let failed = 0;
const check = (what: string, ok: unknown, seen?: unknown) => {
  if (!ok) failed++;
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${what}${ok ? '' : `: ${JSON.stringify(seen)}`}`);
};
const wait = (ms = 400) => page.waitForTimeout(ms);
const shot = (name: string) => page.screenshot({ path: join(shots, 'share-' + name + '.png') });
const until = (fn: string, timeout = 20000) => page.waitForFunction(fn, null, { timeout }).then(() => true, () => false);
const ready = () => until('document.documentElement.dataset.e2e === "ready"', 120000);
const top = () => page.locator('.scrim').last();
const names = async (path: string) => page.evaluate(async (p) => {
  const r = await fetch('/api/v1/fs/list?path=' + encodeURIComponent(p));
  return ((await r.json()).items as { name: string }[]).map((i) => i.name);
}, path);

// What Android does after the person picks CannonCommander in the share sheet: a form POST to the share target.
async function share(fields: Record<string, string>, files: { name: string; mimeType: string; buffer: Buffer }[]) {
  await page.evaluate((f) => {
    const form = Object.assign(document.createElement('form'), { method: 'post', enctype: 'multipart/form-data', action: '/teilen', id: 'test-share' });
    for (const [name, value] of Object.entries(f)) form.append(Object.assign(document.createElement('input'), { name, value }));
    form.append(Object.assign(document.createElement('input'), { type: 'file', name: 'dateien', multiple: true }));
    document.body.append(form);
  }, fields);
  if (files.length) await page.setInputFiles('#test-share input[type=file]', files);
  await Promise.all([page.waitForURL(/\?geteilt|\/$/), page.evaluate(() => (document.getElementById('test-share') as HTMLFormElement).submit())]);
  return ready();
}

await page.goto(app);
check('the shim is ready', await ready());
const worker = await page.evaluate(async () => {
  const r = await navigator.serviceWorker.ready;
  return r.active?.scriptURL ?? '';
});
check('the service worker runs for the whole app', worker.endsWith('/sw.js'), worker);
const manifest = await page.evaluate(async () => (await fetch(document.querySelector<HTMLLinkElement>('link[rel="manifest"]')!.href)).json());
check('the manifest offers the share target', manifest.share_target?.action === 'teilen' && manifest.share_target.method === 'POST', manifest.share_target);

check('the app opens after two files were shared', await share({ titel: 'Urlaub' }, [
  { name: 'strand.jpg', mimeType: 'image/jpeg', buffer: Buffer.alloc(3000, 1) },
  { name: 'notizen.txt', mimeType: 'text/plain', buffer: Buffer.from('Sonnencreme\n') },
]));
check('the address no longer says geteilt', !page.url().includes('geteilt'), page.url());
const asking = (text: string) => until(`[...document.querySelectorAll('.scrim')].some((s) => (s.textContent ?? "").includes(${JSON.stringify(text)}))`);
check('it asks where the files go', await asking('Geteilte Dateien hochladen'));
const rows = await top().locator('.ci-list > *').allInnerTexts();
check('it lists both files', rows.length === 2 && /strand\.jpg/.test(rows[0]!) && /notizen\.txt/.test(rows[1]!), rows);
await shot('ask-dark');
await page.evaluate(() => (window as never as { CC: { setPref(k: string, v: string): void } }).CC.setPref('theme', 'light'));
await wait(300);
await shot('ask-light');
await page.evaluate(() => (window as never as { CC: { setPref(k: string, v: string): void } }).CC.setPref('theme', 'dark'));

await top().locator('#ci-target').fill('/mnt/user/gibt-es-nicht');
await top().locator('.win-foot .btn.primary').click();
await wait(600);
check('a folder that does not exist keeps the window open', await top().locator('#ci-target').count() === 1);
await top().locator('#ci-target').fill('/mnt/user/downloads');
await top().locator('.win-foot .btn.primary').click();
check('the files land in the folder', await page.waitForFunction(async () => {
  const r = await fetch('/api/v1/fs/list?path=' + encodeURIComponent('/mnt/user/downloads'));
  const got = ((await r.json()).items as { name: string }[]).map((i) => i.name);
  return got.includes('strand.jpg') && got.includes('notizen.txt');
}, null, { timeout: 20000 }).then(() => true, () => false), await names('/mnt/user/downloads'));
check('the box is empty afterwards', await page.evaluate(async () => (await (await caches.open('cannoncommander-shared')).keys()).length === 0));

await page.evaluate('while (winStack.length) CC.closeWin()');
await page.reload();
await ready();
await wait(800);
check('a reload offers nothing again', !(await page.evaluate(() => [...document.querySelectorAll('.scrim')].some((s) => s.textContent?.includes('Geteilte Dateien')))));

check('the app opens after a link was shared', await share({ titel: 'Rezept: Kaiserschmarrn', adresse: 'https://example.org/kaiserschmarrn' }, []));
check('a link comes as a text file named after its title', await asking('Rezept Kaiserschmarrn.txt'));
await page.setViewportSize({ width: 390, height: 844 });
await wait(400);
await shot('ask-phone');
await page.setViewportSize({ width: 1600, height: 950 });
await top().locator('#ci-target').fill('/mnt/user/downloads');
await top().locator('.win-foot .btn.primary').click();
check('the text file lands too', await page.waitForFunction(async () => {
  const r = await fetch('/api/v1/fs/list?path=' + encodeURIComponent('/mnt/user/downloads'));
  return ((await r.json()).items as { name: string }[]).some((i) => i.name === 'Rezept Kaiserschmarrn.txt');
}, null, { timeout: 20000 }).then(() => true, () => false));

check('no page errors', errors.length === 0, errors);
await browser.close();
console.log(failed ? `${failed} failed` : 'all passed');
process.exit(failed ? 1 : 0);
