// node tests/parity/settings.ts [app-url] [shots-dir]: every settings tab in dark and light, once in the built
// prototype and once in the app running with `vite --mode fixture`, side by side as screenshots, with the share of
// pixels that differ. Playwright comes from the prototype's own install.
import { mkdirSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..', '..', '..');
const { chromium } = createRequire(join(root, 'prototype', 'package.json'))('playwright');
const app = process.argv[2] || 'http://localhost:5191/';
const shots = resolve(process.argv[3] || join(root, 'prototype', 'shots'));
const prototype = 'file:///' + join(root, 'prototype', 'dist', 'cannoncommander.html').split('\\').join('/');
const TABS = ['general', 'look', 'files', 'sources', 'remotes', 'users', 'rules', 'shares', 'security', 'integrations', 'app', 'info'];
mkdirSync(shots, { recursive: true });

const browser = await chromium.launch();
const errors = [];
async function open(url, theme) {
  const page = await browser.newPage({ viewport: { width: 1600, height: 950 } });
  page.on('pageerror', (e) => errors.push(`${url}: ${String(e).slice(0, 200)}`));
  await page.addInitScript((t) => localStorage.setItem('cannoncommander-prefs', JSON.stringify({ theme: t })), theme);
  await page.goto(url);
  await page.waitForTimeout(900);
  return page;
}
// The share of pixels whose colour differs by more than a little, worked out in a page of its own.
async function differ(a, b) {
  const page = await browser.newPage();
  const share = await page.evaluate(async ([x, y]) => {
    const load = (src) => new Promise((ok) => { const i = new Image(); i.onload = () => ok(i); i.src = src; });
    const [i, j] = await Promise.all([load(x), load(y)]);
    const px = (img) => { const c = new OffscreenCanvas(img.width, img.height), g = c.getContext('2d'); g.drawImage(img, 0, 0); return g.getImageData(0, 0, img.width, img.height).data; };
    const p = px(i), q = px(j);
    let off = 0;
    for (let k = 0; k < p.length; k += 4) if (Math.abs(p[k] - q[k]) + Math.abs(p[k + 1] - q[k + 1]) + Math.abs(p[k + 2] - q[k + 2]) > 48) off++;
    return off / (p.length / 4);
  }, [a, b].map((f) => 'data:image/png;base64,' + readFileSync(f).toString('base64')));
  await page.close();
  return share;
}

// Windows opened the same way in both, each from the tab it belongs to. The accent picker comes last, as it changes the accent.
const WINDOWS = [
  ['users', 'user', (p) => p.locator('[data-uopen]').first().click()],
  ['remotes', 'remote', (p) => p.getByRole('button', { name: 'Fernziel hinzufügen' }).click()],
  ['rules', 'rule', (p) => p.getByRole('button', { name: 'Regel anlegen' }).click()],
  ['security', 'token', (p) => p.getByRole('button', { name: 'Token erstellen' }).click()],
  ['info', 'keys', (p) => p.keyboard.press('F1')],
  ['look', 'accent', (p) => p.locator('[data-preset="1"]').click()],
];
const shoot = async (page, file) => { await page.mouse.move(1590, 940); await page.waitForTimeout(250); await page.screenshot({ path: file }); };
const rows = [];
for (const theme of ['dark', 'light']) {
  const proto = await open(prototype, theme), mine = await open(app, theme);
  await mine.click('[data-go-view="settings"]');
  for (const tab of TABS) {
    await proto.evaluate((id) => CC.go('settings', id), tab);
    await mine.click(`[data-stab="${tab}"]`);
    await Promise.all([proto.waitForTimeout(500), mine.waitForTimeout(700)]);
    const a = join(shots, `w1j1-${tab}-${theme}-prototype.png`), b = join(shots, `w1j1-${tab}-${theme}-app.png`);
    await shoot(proto, a);
    await shoot(mine, b);
    rows.push([tab, theme, (100 * await differ(a, b)).toFixed(1) + ' %']);
  }
  for (const [tab, name, open] of WINDOWS) {
    await proto.evaluate((id) => CC.go('settings', id), tab);
    await mine.click(`[data-stab="${tab}"]`);
    await Promise.all([proto.waitForTimeout(400), mine.waitForTimeout(600)]);
    await open(proto);
    await open(mine);
    await Promise.all([proto.waitForTimeout(500), mine.waitForTimeout(700)]);
    const a = join(shots, `w1j1-win-${name}-${theme}-prototype.png`), b = join(shots, `w1j1-win-${name}-${theme}-app.png`);
    await shoot(proto, a);
    await shoot(mine, b);
    rows.push(['window ' + name, theme, (100 * await differ(a, b)).toFixed(1) + ' %']);
    for (const p of [proto, mine]) await p.keyboard.press('Escape');
    await Promise.all([proto.waitForTimeout(300), mine.waitForTimeout(300)]);
  }
  await proto.close();
  await mine.close();
}
await browser.close();
console.table(rows);
console.log(errors.length ? 'page errors:\n' + errors.join('\n') : 'no page errors');
