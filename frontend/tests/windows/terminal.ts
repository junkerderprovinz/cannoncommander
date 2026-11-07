// node tests/windows/terminal.ts [app-url] [shots-dir]: drives the terminal in the app running with
// `VITE_E2E=1 vite --mode fixture`: the word in the path field while the switch is off, the switch in the settings,
// a terminal that echoes through the fixture's pretend shell, a second one, exit, the folder named like the word, and
// the command in the menus. It changes the fixture's tree, so it wants a fixture server of its own, started fresh.
import { mkdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..', '..', '..');
const { chromium } = createRequire(join(root, 'prototype', 'package.json'))('playwright') as typeof import('playwright');
const app = process.argv[2] || 'http://localhost:5191/';
const shots = resolve(process.argv[3] || join(root, 'testing', 'out', 'windows'));
mkdirSync(shots, { recursive: true });

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1600, height: 950 }, locale: 'de-AT' });
const errors: string[] = [];
page.on('pageerror', (e: Error) => errors.push(String(e).slice(0, 300)));
let failed = 0;
const check = (what: string, ok: unknown, seen?: unknown) => {
  if (!ok) failed++;
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${what}${ok ? '' : `: ${JSON.stringify(seen)}`}`);
};
const wait = (ms = 400) => page.waitForTimeout(ms);
const shot = (name: string) => page.screenshot({ path: join(shots, 'terminal-' + name + '.png') });
const until = (fn: string, timeout = 20000) => page.waitForFunction(fn, null, { timeout }).then(() => true, () => false);
const text = (selector: string) => page.locator(selector).last().innerText().then((s: string) => s.replace(/\s+/g, ' ').trim(), () => '');
const screen = () => text('.bk-term .xterm-rows');
async function at(path: string) {
  await page.evaluate((p) => { const F = (window as never as { Files: { active: number; go(p: string, i: number): void } }).Files; F.active = 0; F.go(p, 0); }, path);
  await page.waitForFunction((p) => (window as never as { Files: { T(i: number): { path: string } } }).Files.T(0).path === p, path);
}
async function typePath(word: string) {
  await page.evaluate('CC.run("gotoPath")');
  await page.waitForSelector('#pathedit-0');
  await page.fill('#pathedit-0', word);
  await page.keyboard.press('Enter');
  await wait(600);
}

await page.goto(app);
check('the shim is ready', await until('document.documentElement.dataset.e2e === "ready"', 120000));
await wait(600);

// While the switch is off the command stays out of every menu, and the word in the path field says how to switch it on.
await at('/mnt/user/appdata');
check('no command while it is off', await page.evaluate('!CC.actions.terminal'));
await typePath('cmd');
check('the word asks for the switch', /noch ausgeschaltet/.test(await text('.toast')), await text('.toast'));
check('no window opens', await page.evaluate('winStack.length === 0'));
await shot('off');
await page.locator('.toast button', { hasText: 'Zu den Einstellungen' }).click();
check('the toast leads to the integrations', await until('CC.view === "settings" && CC.settingsTab_ === "integrations"'), await page.evaluate('CC.view + " " + CC.settingsTab_'));
await page.locator('[data-it="terminal"]').scrollIntoViewIfNeeded();
await page.click('[data-it="terminal"]');
check('switching it on brings the command', await until('!!CC.actions.terminal'));
await shot('switch');

// On, a word in any case opens a terminal in the folder of the panel, and the pretend shell answers.
await page.evaluate('CC.go("files")');
await at('/mnt/user/appdata');
await typePath('PowerShell');
check('a terminal window opens', await until('!!document.querySelector(".bk-term .xterm-rows")'));
check('the shell starts in the folder', await until('/fixture shell in \\/mnt\\/user\\/appdata/.test(document.querySelector(".bk-term .xterm-rows")?.innerText || "")'), await screen());
check('the folder stands in the foot', /\/mnt\/user\/appdata/.test(await text('.bk-term .start')), await text('.bk-term .start'));
await page.keyboard.type('echo hallo welt');
await page.keyboard.press('Enter');
check('typing goes there and back', await until('/^hallo welt$/m.test(document.querySelector(".bk-term .xterm-rows")?.innerText || "")'), await screen());
await page.keyboard.press('Escape');
await wait(300);
check('Escape belongs to the shell', await page.evaluate('winStack.length === 1'));
await shot('open');

await page.locator('.bk-term .win-foot .btn', { hasText: 'Noch ein Terminal' }).click();
check('a second terminal opens over the first', await until('winStack.length === 2 && document.querySelectorAll(".bk-term .xterm-rows").length === 2'));
await page.evaluate('CC.closeWin()');
check('closing it leaves the first', await until('winStack.length === 1'));
await page.locator('.bk-term .xterm').last().click();
await page.keyboard.type('exit');
await page.keyboard.press('Enter');
check('exit closes the window', await until('winStack.length === 0'));

// A folder here that is named like the word wins over the terminal.
await page.evaluate(() => fetch('/api/v1/fs/mkdir', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path: '/mnt/user/appdata/bash' }) }));
await page.evaluate('CC.run("refresh")');
await page.waitForFunction(() => (window as never as { Files: { entries(i: number): { name: string }[] } }).Files.entries(0).some((e) => e.name === 'bash'));
await typePath('bash');
check('the folder opens instead', await until('Files.T(0).path === "/mnt/user/appdata/bash"'), await page.evaluate('Files.T(0).path'));
check('and no terminal', await page.evaluate('winStack.length === 0'));

// The command in the background menu of the list and in the command search.
await at('/mnt/user/appdata');
const list = page.locator('.panel.active .list');
const box = (await list.boundingBox())!;
await list.click({ button: 'right', position: { x: 40, y: box.height - 20 } });
check('the background menu offers it', /Terminal hier öffnen/.test(await text('#menu')), await text('#menu'));
await shot('menu');
await page.keyboard.press('Escape');
await page.evaluate('CC.run("palette")');
await page.keyboard.type('Terminal');
await wait(400);
check('the command search finds it', /Terminal hier öffnen/.test(await text('.glim-window, .scrim')), await text('.scrim'));
await page.evaluate('CC.closeWin(); closeMenu()');

check('no page errors', errors.length === 0, errors);
await browser.close();
console.log(failed ? `${failed} checks failed` : 'all checks passed');
process.exit(failed ? 1 : 0);
