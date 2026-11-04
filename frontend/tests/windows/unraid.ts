// node tests/windows/unraid.ts [app-url] [shots-dir]: drives the Unraid parts in the app running with
// `VITE_E2E=1 vite --mode fixture`: sleeping disks in the tree, the share's storage in the panel foot, files still on
// the cache, the parity run in the sidebar and the rights repair. Keeps a screenshot of each.
// It changes the fixture's tree and disks as it goes, so it wants a fixture server of its own, started fresh.
import { mkdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..', '..', '..');
const { chromium } = createRequire(join(root, 'prototype', 'package.json'))('playwright');
const app = process.argv[2] || 'http://localhost:5191/';
const shots = resolve(process.argv[3] || join(root, 'testing', 'out', 'unraid'));
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
const shot = (name: string) => page.screenshot({ path: join(shots, name + '.png') });
const until = (fn: string, timeout = 20000) => page.waitForFunction(fn, null, { timeout }).then(() => true, () => false);
const primary = () => page.locator('.scrim').last().locator('.win-foot .btn.primary').click();
const closeAll = () => page.evaluate('while (winStack.length) CC.closeWin(); closeMenu(); document.querySelectorAll(".toast").forEach((t) => t.remove())');
const toasts = () => page.evaluate('[...document.querySelectorAll(".toast")].map((t) => t.textContent.trim()).join(" | ")') as Promise<string>;
const moons = () => page.evaluate('[...document.querySelectorAll("#tree .u-moon")].map((m) => m.closest(".nav-row").dataset.go).join(", ")') as Promise<string>;
const names = (panel = 0) => page.evaluate(`Files.entries(${panel}).map((k) => k.name)`) as Promise<string[]>;
async function at(path: string, name?: string, panel = 0) {
  await page.evaluate(([p, i]) => { const F = (window as never as { Files: { active: number; go(p: string, i: number): void } }).Files; F.active = i as number; F.go(p as string, i as number); }, [path, panel] as const);
  await page.waitForFunction(([p, i, n]) => {
    const F = (window as never as { Files: { T(i: number): { path: string; cursor: number }; entries(i: number): { name: string }[] } }).Files;
    const k = n ? F.entries(i as number).findIndex((e) => e.name === n) : 0;
    if (F.T(i as number).path !== p || k < 0) return false;
    F.T(i as number).cursor = k;
    return true;
  }, [path, panel, name ?? ''] as const);
}
const api = (method: string, path: string, body?: unknown) => page.evaluate(([m, p, b]) => fetch('/api/v1' + p, { method: m as string, headers: { 'Content-Type': 'application/json' }, body: b ? JSON.stringify(b) : undefined }).then((r) => r.json()), [method, path, body] as const);

await page.goto(app);
check('the shim is ready', await until('document.documentElement.dataset.e2e === "ready"', 120000));
await wait(600);

// Sleeping disks: a moon in the tree, a word before a read wakes one, a disk that falls asleep on the server, all to sleep.
await page.evaluate('Files.openTree.add("/mnt")');
check('disk 1 and disk 3 sleep at the start', await until('[...document.querySelectorAll("#tree .u-moon")].length === 2'), await moons());
check('the moons sit on the disk rows', (await moons()) === '/mnt/disk1, /mnt/disk3', await moons());
await page.evaluate('document.querySelector("#tree [data-go=\\"/mnt/disk3\\"]").scrollIntoView({ block: "end" })');
await shot('tree-moons');
await at('/mnt/user/filme/Big Buck Bunny (2008)', 'poster.jpg');
await page.evaluate('CC.run("view")');
check('opening a file on a sleeping disk says it wakes', await until('document.querySelector(".toast")?.textContent.includes("Disk 1 schläft und fährt jetzt hoch")'), await toasts());
check('disk 1 lost its moon', (await moons()) === '/mnt/disk3', await moons());
await wait(800);
await shot('wake');
await closeAll();
await api('PUT', '/fixture/spin', { disk: 'disk2', asleep: true });
check('a disk that falls asleep on the server gets its moon', await until('document.querySelector("#tree [data-go=\\"/mnt/disk2\\"] .u-moon")'), await moons());
check('spinning down is in the tools menu', await page.evaluate('MENUS.find((m) => m[2] === "tools")[1].includes("spinDown")'));
await page.evaluate('CC.run("spinDown")');
check('every disk sleeps after spinning down', await until('document.querySelectorAll("#tree .u-moon").length === 3'), await moons());
check('spinning down says so', /Disk 1 schläft jetzt|schlafen jetzt/.test(await toasts()), await toasts());
await page.evaluate('document.querySelector("#tree [data-go=\\"/mnt/disk3\\"]").scrollIntoView({ block: "end" })');
await shot('tree-asleep');
await closeAll();

// Cache or array: the share's pill in the foot, cache pills on what still waits for the mover, the cache-only view.
await at('/mnt/user/downloads');
check('the foot names the way of the mover', await until('document.querySelector(".panel.active .u-store")?.textContent === "Cache → Array"'),
  await page.evaluate('document.querySelector(".panel.active .u-store")?.textContent'));
check('its tip says when the mover runs and what still waits', await until('document.querySelector(".panel.active .u-store").dataset.tip.includes("19 Dateien")'),
  await page.evaluate('document.querySelector(".panel.active .u-store").dataset.tip'));
check('the tip reads as in the prototype', (await page.evaluate('document.querySelector(".panel.active .u-store").dataset.tip')) === 'Share downloads: Primärspeicher Cache, Sekundärspeicher Array. Neue Dateien landen auf dem Cache, der Mover schiebt sie um 3:40 Uhr auf Disk 1. Hier liegen noch 19 Dateien mit 4,3 GB auf dem Cache.',
  await page.evaluate('document.querySelector(".panel.active .u-store").dataset.tip'));
await until('document.querySelectorAll(".panel.active .u-cache").length >= 5');
const cached = await page.evaluate('[...document.querySelectorAll(".panel.active .u-cache")].map((b) => b.closest(".name").querySelector(".t").textContent).join(", ")');
check('files still on the cache carry a pill', cached === 'unfertig, album.flac, debian-13.1.0-amd64-netinst.iso, Rechnung-Oktober.pdf, setup-notes.md', cached);
await page.hover('.panel.active .u-store');
await wait(700);
await shot('foot-cache');
await page.click('[data-menu="2"]');
await wait();
check('the view menu has the cache filter', await page.locator('#menu button:has-text("Nur auf dem Cache")').count() === 1);
await page.click('#menu button:has-text("Nur auf dem Cache")');
check('the filter keeps only what lies on the cache', await until('Files.entries().map((k) => k.name).join() === "unfertig,album.flac,debian-13.1.0-amd64-netinst.iso,Rechnung-Oktober.pdf,setup-notes.md"'), await names());
check('the filter says how many', (await toasts()).includes('5 auf dem Cache'), await toasts());
await shot('cache-only');
await page.click('.panel.active .u-filter');
check('the pill in the foot turns the filter off', await until('Files.entries().length === 9'), await names());
await closeAll();
await at('/mnt/user/filme');
check('an array-only share says so', await until('document.querySelector(".panel.active .u-store")?.textContent.trim() === "nur Array"'));
await at('/mnt/disk1/filme');
check('a disk share warns', await until('document.querySelector(".panel.active .u-store")?.textContent.trim() === "Disk-Share"'));
check('and says the disk sleeps', (await page.evaluate('document.querySelector(".panel.active .u-store").dataset.tip')).includes('Die Platte schläft'));
await page.hover('.panel.active .u-store');
await wait(700);
await shot('foot-disk-share');

// The disk column shows where each file lies under its label.
await page.evaluate('CC.setPref("columns", ["name", "size", "disk"])');
await at('/mnt/user/downloads');
check('the disk column names cache and disks', await until('[...document.querySelectorAll(".panel.active .row")].some((r) => r.textContent.includes("album.flac") && r.textContent.includes("Cache"))'));
check('a folder lists every disk it spreads over', await page.evaluate('[...document.querySelectorAll(".panel.active .row")].some((r) => r.textContent.includes("unfertig") && r.textContent.includes("Cache"))'));
await shot('disk-column');
await page.evaluate('CC.setPref("columns", ["name", "size", "modified"])');

// The parity run in the sidebar, with its end reported.
await api('PUT', '/fixture/parity', { busy: true });
check('the parity check shows in the sidebar', await until('document.querySelector("[data-parity]")?.textContent.includes("Paritätsprüfung")'));
check('its percentage moves on', await until('parseInt(document.querySelector("[data-parity] .meta").textContent) >= 4', 15000), await page.evaluate('document.querySelector("[data-parity]")?.textContent'));
await shot('parity');
await api('PUT', '/fixture/parity', { busy: false, finished: true });
check('the row leaves when the check ends', await until('!document.querySelector("[data-parity]")'));
check('the end is reported', (await toasts()).includes('Die Paritätsprüfung ist fertig'), await toasts());
await closeAll();

// The rights repair: refused in appdata, the kept folders listed for all shares, then run as a job.
await at('/mnt/user', 'appdata');
await page.click('.panel.active .row.cursor', { button: 'right' });
await wait();
check('the row menu offers the repair', await page.locator('#menu button:has-text("Rechte wie Unraid setzen")').count() === 1);
await page.click('#menu button:has-text("Rechte wie Unraid setzen")');
check('appdata keeps its owners', await until('document.querySelector(".scrim .u-alert b")?.textContent === "Hier setzt CannonCommander keine Rechte"'));
await wait(800);
await shot('perms-appdata');
await closeAll();
await at('/mnt', 'user');
await page.evaluate('CC.run("fixPerms")');
check('the window lists the folders it leaves alone', await until('document.querySelectorAll(".scrim .u-tbl tr").length === 2'));
const kept = await page.evaluate('[...document.querySelectorAll(".scrim .u-tbl tr")].map((r) => r.textContent.trim()).join(" | ")');
check('appdata and system stay as they are', kept === '/mnt/user/appdataContainer-Daten101 MB | /mnt/user/systemDocker und VMs41 GB', kept);
await wait(800);
await shot('perms-window');
await primary();
check('the repair runs and reports what it set', await until('document.querySelector(".toast") && [...document.querySelectorAll(".toast")].some((t) => /Rechte gesetzt für \\d+ Objekte, 2 Ordner ausgelassen/.test(t.textContent))', 30000), await toasts());
const after = await api('POST', '/unraid/fixperms/scan', { path: '/mnt/user/downloads' });
check('the files carry the new rights', after.wrong === 0, after);
await shot('perms-done');

check('no errors on the page', !errors.length, errors);
await browser.close();
console.log(failed ? `${failed} failed` : 'all passed');
process.exit(failed ? 1 : 0);
