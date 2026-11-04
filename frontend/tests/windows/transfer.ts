// node tests/windows/transfer.ts [app-url] [shots-dir]: drives the transfer, conflict, jobs, trash, upload, undo,
// naming and gather windows in the app running with `VITE_E2E=1 vite --mode fixture`, checks what each one does to the
// fixture's tree and keeps a screenshot of each. Playwright comes from the prototype's own install.
// It changes the tree as it goes, so it wants a fixture server of its own, started fresh.
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
const shot = (name: string) => page.screenshot({ path: join(shots, name + '.png') });
const until = (fn: string, timeout = 20000) => page.waitForFunction(fn, null, { timeout }).then(() => true, () => false);
const primary = () => page.locator('.scrim').last().locator('.win-foot .btn.primary').click();
const closeAll = () => page.evaluate('while (winStack.length) CC.closeWin(); closeMenu()');
const names = (panel = 0) => page.evaluate(`Files.entries(${panel}).map((k) => k.name)`) as Promise<string[]>;
// Opens the folder in a panel and waits until it is read, then puts the cursor and the marks on the names.
async function at(path: string, picked: string[], panel = 0) {
  await page.evaluate(([p, i]) => { const F = (window as never as { Files: { active: number; go(p: string, i: number): void } }).Files; F.active = i as number; F.go(p as string, i as number); }, [path, panel] as const);
  await page.waitForFunction(([p, i, n]) => {
    const F = (window as never as { Files: { T(i: number): { path: string; marked: Set<string>; cursor: number }; entries(i: number): { name: string }[] } }).Files;
    const k = F.entries(i as number).findIndex((e) => e.name === (n as string[])[0]);
    if (F.T(i as number).path !== p || k < 0) return false;
    F.T(i as number).cursor = k;
    F.T(i as number).marked = new Set((n as string[]).length > 1 ? (n as string[]) : []);
    return true;
  }, [path, panel, picked] as const);
}
const api = (method: string, path: string, body?: unknown) => page.evaluate(([m, p, b]) => fetch('/api/v1' + p, { method: m as string, headers: { 'Content-Type': 'application/json' }, body: b ? JSON.stringify(b) : undefined }).then((r) => r.json()), [method, path, body] as const);

await page.goto(app);
check('the shim is ready', await until('document.documentElement.dataset.e2e === "ready"', 120000));
await wait(600);

// F5 opens the copy window with the other panel as target, the conflict window asks, the job runs and ends.
await page.evaluate('Files.go("/mnt/user/backups", 1)');
await at('/mnt/user/downloads', ['ubuntu-24.04.3-desktop-amd64.iso', 'album.flac']);
await wait();
await page.keyboard.press('F5');
await wait(700);
check('copy window aims at the other panel', (await page.inputValue('#dest')) === '/mnt/user/backups', await page.inputValue('#dest'));
check('copy window asks by default', (await page.getAttribute('#clash', 'data-val')) === 'ask');
await shot('copy');
await primary();
await wait(800);
check('album.flac is a hard link, so the copy asks first', (await page.locator('.scrim .notch').last().textContent()) === 'Harte Links');
await shot('links');
await page.locator('.scrim').last().locator('.win-foot .btn', { hasText: 'Trotzdem kopieren' }).click();
await wait(800);
check('the conflict window lists the name that exists', (await page.locator('.cf-row b').allInnerTexts()).join() === 'ubuntu-24.04.3-desktop-amd64.iso', await page.locator('.cf-row b').allInnerTexts());
await shot('conflicts');
await page.click('#cf-all button[data-v="both"]');
await primary();
await wait(300);
await page.evaluate('CC.run("jobs")');
await wait(500);
check('the job shows in the jobs window', /Kopieren: 2 Objekte nach backups/.test(await page.locator('#jobs-body .job').first().innerText()));
await shot('jobs-running');
check('the job ends', await until('Jobs.list[0]?.state === "fertig"'));
await closeAll();
await wait(800);
check('both copies are there', (await names(1)).includes('ubuntu-24.04.3-desktop-amd64 (1).iso'), await names(1));

// Strg+Z takes the copy back.
await page.evaluate('document.querySelector(".panel.active .list")?.focus()');
await page.evaluate('CC.run("undoList")');
await wait(300);
await shot('undo-list');
check('the undo list holds the copy once', (await page.evaluate('CC.history.list.filter((s) => /kopiert|Kopiert/.test(s.label)).length')) === 1);
await page.keyboard.press('Escape');
await page.keyboard.press('Control+z');
check('Strg+Z took the copy back', await until('!Files.entries(1).some((k) => k.name === "ubuntu-24.04.3-desktop-amd64 (1).iso") && Files.entries(1).every((k) => k.name !== "album.flac")'));

// Jobs: pause, resume, order, cancel and retry.
await api('PUT', '/jobs/settings', { mode: 'serial', cap: 0, parityWait: true, notifyUnraid: false, notifyAfter: 60 });
for (const f of ['appdata-2026-10-24.tar.zst', 'vm-win11-2026-10-01.img', 'flash-2026-10-31.zip']) {
  await api('POST', '/jobs', { kind: 'copy', params: { sources: ['/mnt/user/backups/' + f], dest: '/mnt/user/dokumente', rule: 'both' }, cap: 5 * 1024 * 1024 });
}
await page.evaluate('CC.run("jobs")');
await wait(800);
const order = () => page.evaluate('Jobs.running().map((j) => j.job.name + " " + j.state).join(" | ")') as Promise<string>;
check('serial: one runs, two wait', /läuft \| .* wartet \| .* wartet/.test(await order()), await order());
await page.click('#jobs-body .job:nth-child(3) [data-jact="up"]');
await wait(600);
check('the third moved up one place', (await order()).split(' | ')[1]!.startsWith('flash'), await order());
await page.click('#jobs-body .job:nth-child(1) [data-jact="pause"]');
await wait(600);
check('pause holds the first job', (await order()).split(' | ')[0]!.endsWith('pausiert'), await order());
await shot('jobs-queue');
await page.click('#jobs-body .job:nth-child(1) [data-jact="limit"]');
await wait(300);
await shot('jobs-limit');
await page.keyboard.press('Escape');
for (let i = 0; i < 3; i++) {
  await page.locator('#jobs-body .job.open [data-jact="cancel"]').first().click();
  await wait(400);
}
check('cancel ends every open job', (await page.evaluate('Jobs.running().length')) === 0);
await page.locator('#jobs-body [data-jact="retry"]').first().click();
await wait(500);
check('retry puts a cancelled job back in the queue', (await page.evaluate('Jobs.running().length')) === 1);
await page.locator('#jobs-body .job.open [data-jact="cancel"]').first().click();
await wait(300);
await closeAll();

// A job that meets a name on its way asks in the conflict window and goes on with the answer.
await api('POST', '/fs/touch', { path: '/mnt/user/dokumente/setup-notes.md' });
await api('POST', '/jobs', { kind: 'copy', params: { sources: ['/mnt/user/downloads/setup-notes.md'], dest: '/mnt/user/dokumente', rule: 'ask' } });
check('the asking job opens the conflict window', await until('[...document.querySelectorAll(".scrim .notch")].some((n) => n.textContent === "Konflikte")'));
await shot('conflict-running');
await page.click('.cf-row .seg button[data-v="skip"]');
await primary();
check('the job ends after the answer', await until('Jobs.list.find((j) => j.kind === "copy")?.state === "fertig"'));
await closeAll();

// The parity check: a small copy asks whether to wait.
await api('PUT', '/fixture/parity', { busy: true });
await at('/mnt/user/downloads', ['setup-notes.md']);
await page.evaluate('Files.go("/mnt/user/fotos", 1)');
await wait(500);
await page.keyboard.press('F5');
await wait(500);
await primary();
check('the parity question comes up', await until('[...document.querySelectorAll(".scrim .notch")].some((n) => n.textContent === "Paritätsprüfung läuft")'));
await shot('parity');
await page.locator('.scrim').last().locator('.win-foot .btn', { hasText: 'Danach' }).click();
check('the copy waits for the parity check', await until('Jobs.running()[0]?.job.reason === "parity"'));
await api('PUT', '/fixture/parity', { busy: false });
check('it runs once the check is done', await until('Jobs.running().length === 0'));
await closeAll();

// Delete into the trash, the trash page, restore and empty; Umschalt+Entf deletes for good.
await at('/mnt/user/dokumente', ['Einkaufsliste.md']);
await wait();
await page.keyboard.press('F8');
await wait(300);
check('the delete window says where it goes', /landet im Papierkorb/.test(await page.locator('#xwhere').innerText()));
await page.click('#totrash');
await wait(200);
check('without the switch it deletes for good', (await page.locator('.scrim .win-foot .btn.primary').innerText()).includes('Endgültig löschen'));
await shot('delete-forever');
await page.click('#totrash');
await primary();
check('the file went into the trash', await until('DATA.trash.some((e) => e.name === "Einkaufsliste.md")'));
await page.evaluate('CC.run("openTrash")');
await wait(600);
await page.click('.ct-row[data-i="0"]');
await page.click('.ct-row[data-i="1"]', { modifiers: ['Control'] });
await shot('trash');
await page.click('#ct-restore');
check('restore brings it back', await until('!DATA.trash.some((e) => e.name === "Einkaufsliste.md")'));
await page.click('#ct-more');
await wait(300);
await page.locator('button, [role=menuitem]', { hasText: 'Papierkorb leeren' }).last().click();
await wait(300);
await shot('trash-empty-ask');
await primary();
check('emptying leaves nothing', await until('DATA.trash.length === 0'));
await page.evaluate('CC.go("files")');
await at('/mnt/user/dokumente', ['notizen.txt']);
await wait();
await page.keyboard.press('Shift+Delete');
await wait(300);
check('Umschalt+Entf asks with the switch off', (await page.getAttribute('#totrash', 'aria-checked')) === 'false');
await closeAll();

// Upload with a clash.
await at('/mnt/user/downloads', ['setup-notes.md']);
await page.evaluate('CC.run("upload")');
await wait(300);
await page.setInputFiles('#cu-in', [
  { name: 'setup-notes.md', mimeType: 'text/markdown', buffer: Buffer.from('# Neu\n') },
  { name: 'urlaub-video.mp4', mimeType: 'video/mp4', buffer: Buffer.alloc(3 * 1024 * 1024) },
]);
await wait(600);
check('the upload asks about the name that exists', (await page.locator('.cf-row b').allInnerTexts()).join() === 'setup-notes.md');
await page.click('#cf-all button[data-v="both"]');
await primary();
await wait(500);
await shot('upload-progress');
check('both files land', await until('Files.entries(0).some((k) => k.name === "setup-notes (1).md") && Files.entries(0).some((k) => k.name === "urlaub-video.mp4")'));
await shot('upload-done');
await closeAll();

// New folder and rename.
await page.evaluate('document.querySelector(".panel.active .list")?.focus()');
await page.keyboard.press('F7');
await wait(300);
await page.fill('#nm', 'Projekt');
await page.keyboard.press('Enter');
check('F7 makes the folder and puts the cursor on it', await until('Files.entries()[Files.T().cursor]?.name === "Projekt"'));
await page.keyboard.press('F2');
await wait(300);
await page.fill('#nm', 'Projekt 2026');
await page.keyboard.press('Enter');
check('F2 renames it', await until('Files.entries().some((k) => k.name === "Projekt 2026")'));

// Gather a folder.
await at('/mnt/user', ['filme']);
await page.evaluate('CC.run("gather")');
check('the gather plan suggests a disk', await until('/^Verschiebt/.test(document.querySelector("#g-plan")?.textContent || "")'));
await shot('gather');
await primary();
check('the gather job ends', await until('Jobs.list.find((j) => j.kind === "gather")?.state === "fertig"'));

check('no page errors', !errors.length, errors);
await browser.close();
console.log(failed ? `${failed} checks failed` : 'all checks passed');
process.exit(failed ? 1 : 0);
