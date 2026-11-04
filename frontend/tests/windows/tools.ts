// node tests/windows/tools.ts [app-url] [shots-dir]: drives the search window, the saved searches and the search
// field's menu, folder compare and sync, file compare, duplicates and disk usage in the app running with
// `VITE_E2E=1 vite --mode fixture`, checks what each one shows and does, and keeps a screenshot of each.
// It changes the fixture's tree as it goes, so it wants a fixture server of its own, started fresh.
import { mkdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..', '..', '..');
const { chromium } = createRequire(join(root, 'prototype', 'package.json'))('playwright');
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
const shot = (name: string) => page.screenshot({ path: join(shots, 'tools-' + name + '.png') });
const until = (fn: string, timeout = 20000) => page.waitForFunction(fn, null, { timeout }).then(() => true, () => false);
const top = () => page.locator('.scrim').last();
const primary = () => top().locator('.win-foot .btn.primary').click();
const closeAll = () => page.evaluate('while (winStack.length) CC.closeWin(); closeMenu()');
// The search field and its menu, opened afresh as a click into the empty field opens it.
const fieldMenu = async () => {
  await page.evaluate('closeMenu(); Object.assign(Files.T(), { qf: false, filter: "" })');
  await wait(200);
  await page.click('.panel.active [data-run="quickFilter"]');
  await wait(400);
};
const text = (selector: string) => page.locator(selector).first().innerText().then((s: string) => s.replace(/\s+/g, ' ').trim(), () => '');
async function at(path: string, panel = 0, name?: string) {
  await page.evaluate(([p, i]) => { const F = (window as never as { Files: { active: number; go(p: string, i: number): void } }).Files; F.active = i as number; F.go(p as string, i as number); }, [path, panel] as const);
  await page.waitForFunction(([p, i, n]) => {
    const F = (window as never as { Files: { T(i: number): { path: string; cursor: number; marked: Set<string> }; entries(i: number): { name: string }[] } }).Files;
    if (F.T(i as number).path !== p) return false;
    F.T(i as number).marked = new Set();
    if (!n) return true;
    const k = F.entries(i as number).findIndex((e) => e.name === n);
    if (k < 0) return false;
    F.T(i as number).cursor = k;
    return true;
  }, [path, panel, name ?? ''] as const);
}

await page.goto(app);
check('the shim is ready', await until('document.documentElement.dataset.e2e === "ready"', 120000));
await wait(600);

// Suchen: the window grows once there are hits, Enter opens the folder of the hit under the cursor.
await at('/mnt/user/downloads');
await page.keyboard.press('Alt+F7');
await wait(300);
check('the search window starts short', (await page.locator('.bk-search .bk-hits').getAttribute('hidden')) !== null);
await page.keyboard.type('*.iso');
check('hits follow the typing', await until('document.querySelector("#s-count")?.textContent === "2 Treffer"'), await text('#s-count'));
check('the window grew', await page.locator('.win.bk-search.bk-open').count() === 1);
check('the foot counts what was searched', /Objekte durchsucht/.test(await text('.bk-search .win-foot .start')), await text('.bk-search .win-foot .start'));
await shot('search');
await page.fill('#s-name', 'jahr:>2000 *.mp3');
check('field tests find music by year', await until('document.querySelector("#s-count")?.textContent === "2 Treffer"'), await text('#s-count'));
await page.fill('#s-name', 'farbtiefe:>8 foo:1');
check('an unknown field is named', await until('/„foo:“ kenne ich nicht/.test(document.querySelector("#s-list")?.innerText || "")'), await text('#s-list'));
await page.fill('#s-in', '/mnt/user/gibtsnicht');
await page.fill('#s-name', '*.iso');
check('a missing folder says so', await until('/Diesen Ordner gibt es nicht/.test(document.querySelector("#s-list")?.innerText || "")'), await text('#s-list'));
await page.fill('#s-in', '/mnt/user');
await page.fill('#s-name', '');
await page.fill('#s-text', 'plex');
check('a text search runs as a job and lists its hits', await until('/Treffer/.test(document.querySelector("#s-count")?.textContent || "") && document.querySelectorAll("#s-list .bk-hit").length >= 2'), await text('#s-count'));
await shot('search-text');
await page.fill('#s-text', '');
await page.fill('#s-name', '*.iso');
await until('document.querySelector("#s-count")?.textContent === "6 Treffer"');
await page.keyboard.press('ArrowDown');
await page.keyboard.press('Enter');
await wait(600);
check('Enter opens the folder of the second hit with the cursor on it', await until('Files.T().path === "/mnt/user/isos" && Files.cursorEntry()?.name === "debian-13.1.0-amd64-netinst.iso"'), await page.evaluate('Files.T().path + " " + Files.cursorEntry()?.name'));

// Saved searches: save from the window, open from the field's menu, rename and remove by right click.
await page.evaluate('CC.run("search")');
await wait(300);
await page.fill('#s-in', '/mnt/user');
await page.fill('#s-name', '*.mkv');
await page.waitForSelector('.bk-hits:not([hidden]) #s-save');
await page.click('#s-save');
await page.waitForSelector('#sm-name');
check('the name suggested is the pattern', (await page.inputValue('#sm-name')) === '*.mkv');
await page.fill('#sm-name', 'Alle Filme');
await primary();
await wait(400);
await closeAll();
await fieldMenu();
check('the field menu lists the saved searches', /Alle Filme/.test(await text('.qdrop')), await text('.qdrop'));
check('the field menu has its examples', (await page.locator('.qdrop [data-example]').count()) === 6);
await shot('search-menu');
await page.click('.qdrop .qrow:has-text("Alle Filme")');
check('the panel lists the hits of the saved search', await until('/Gespeicherte Suche Alle Filme, 11 Treffer in \\/mnt\\/user/.test(document.querySelector(".panel.active .smart-bar")?.innerText || "")'), await text('.panel.active .smart-bar'));
check('the tab carries the name of the search', /Alle Filme/i.test(await text('.panel.active .ptab.cur')), await text('.panel.active .ptab.cur'));
await shot('smart');
await fieldMenu();
await page.click('.qdrop .qrow:has-text("Alle Filme")', { button: 'right' });
await page.click('#menu button:has-text("Umbenennen")');
await page.fill('#sm-name', 'Filme überall');
await page.keyboard.press('Enter');
await wait(400);
check('rename reaches the tab', await until('/Filme überall/i.test(document.querySelector(".panel.active .ptab.cur")?.innerText || "")'));
await fieldMenu();
await page.click('.qdrop .qrow:has-text("Filme überall")', { button: 'right' });
await page.click('#menu button:has-text("Entfernen")');
check('remove takes the search off the panel', await until('!Files.T().smart && !document.querySelector(".smart-bar")'));
await fieldMenu();
await page.click('.qdrop [data-example="*.mkv"]');
check('an example fills the field and keeps the focus', await until('Files.T().filter === "*.mkv" && document.activeElement?.id === "qf-0"'));
await page.click('.panel.active [data-qfclose]');

// Ordner vergleichen and Synchronisieren between the two panels.
await page.evaluate('CC.setPref("dual", true)');
await at('/mnt/user/backups/dokumente', 1);
await at('/mnt/user/dokumente', 0);
await page.keyboard.press('Control+Shift+C');
await wait(500);
check('compare counts the states', /Gleich 4 Links neuer 1 Rechts neuer 2 Nur links 3 Nur rechts 1/.test(await text('.scrim .tbl')), await text('.scrim .tbl'));
const marks = (panel: number) => page.locator(`.panel[data-panel="${panel}"] .pill.state`).allInnerTexts().then((all: string[]) => all.map((s) => s.toLowerCase()));
check('compare marks the rows in both panels', (await marks(0)).includes('neuer') && (await marks(1)).includes('nur hier'), [await marks(0), await marks(1)]);
await shot('compare');
await primary();
await wait(800);
check('sync plans to the right', /Schritte von/i.test(await text('#y-count')), await text('#y-count'));
check('a newer target starts switched off', (await page.locator('#y-list .bk-step.skip').count()) >= 1);
await shot('sync');
await page.click('#y-dir [data-v="both"]');
await wait(500);
check('both ways turns mirroring off', (await page.getAttribute('#y-mirror', 'aria-checked')) === 'false' && (await page.locator('#y-mirror').isDisabled()));
await shot('sync-both');
await page.click('#y-dir [data-v="right"]');
await wait(500);
await top().locator('.win-foot .btn', { hasText: 'In ArrowLoop planen' }).click();
await wait(300);
check('the ArrowLoop window suggests a name', (await page.inputValue('#al-name')) === 'dokumente nach dokumente', await page.inputValue('#al-name'));
await primary();
check('the hand over to ArrowLoop is confirmed', await until('[...document.querySelectorAll(".toast")].some((t) => /in ArrowLoop geplant/.test(t.textContent))'));
await primary();
check('sync runs as a job and ends', await until('Jobs.list.some((j) => j.kind === "sync" && j.state === "fertig")'), await page.evaluate('Jobs.list.map((j) => j.kind + " " + j.state)'));
await closeAll();
const after = await page.evaluate(`fetch('/api/v1/tools/compare', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ left: '/mnt/user/dokumente', right: '/mnt/user/backups/dokumente', top: true }) }).then((r) => r.json()).then((b) => b.counts)`) as Record<string, number>;
check('after the sync only the newer targets differ', !after.onlyL && !after.newerL, after);

// Dateien vergleichen: the file under the cursor in each panel.
await at('/mnt/user/backups/dokumente', 1, 'docker-compose.yml');
await at('/mnt/user/dokumente', 0, 'docker-compose.yml');
await page.evaluate('CC.run("diff")');
await wait(600);
check('diff counts its places', /\d+ Stellen?: \d+ geändert, \d+ entfernt, \d+ dazu/.test(await text('.bk-diff .win-foot .start')), await text('.bk-diff .win-foot .start'));
await shot('diff');
await primary();
await primary();
check('next place jumps', /Stelle 2 von/.test(await text('.bk-diff .win-foot .start')) && (await page.locator('.bk-dl.cur').count()) === 1, await text('.bk-diff .win-foot .start'));
await shot('diff-next');
await closeAll();

// Doppelte Dateien: the job finds the copies, keeping the newest picks the rest, deleting puts them in the trash.
await at('/mnt/user', 0);
await page.evaluate('CC.run("duplicates")');
check('duplicates finds groups', await until('/Gruppen, .* doppelt/.test(document.querySelector("#u-sum")?.textContent || "")'), await text('#u-sum'));
await shot('dups');
await page.click('[data-bk-keep="new"]');
await wait(200);
const label = await text('.bk-dups .win-foot .btn.primary');
check('keeping the newest picks the other copies', /Kopien löschen, /.test(label), label);
await shot('dups-keep');
await primary();
check('the copies went to the trash', await until('[...document.querySelectorAll(".toast")].some((t) => /im Papierkorb, .* frei/.test(t.textContent))'));
check('the groups are gone', await until('/Keine doppelten Dateien gefunden/.test(document.querySelector("#u-list")?.innerText || "")'), await text('#u-list'));
await closeAll();

// Speicherbelegung: the treemap first, the tree with its sorting, picking and deleting, types and largest files. The
// first click on Name sorts from Z, as in the prototype.
await page.evaluate('CC.run("diskUsage", "/mnt/user")');
await wait(800);
check('disk usage opens on the treemap', (await page.getAttribute('#du-view [aria-pressed="true"]', 'data-v')) === 'map' && (await page.locator('#du-body .bk-blk').count()) > 3);
await shot('usage');
await page.click('.bk-blk.dir');
await wait(600);
check('a click on a folder block goes into it', /Server › mnt › user › \S+/.test(await text('#du-crumbs')), await text('#du-crumbs'));
await page.goBack();
await wait(400);
check('the side button goes back', (await text('#du-crumbs')) === 'Server › mnt › user', await text('#du-crumbs'));
await page.click('#du-view [data-v="tree"]');
await wait(400);
check('the tree lists the biggest first', (await text('#du-body .du-row:not(.du-head)')).startsWith('backups'), await text('#du-body .du-row:not(.du-head)'));
await page.click('#du-body .du-row:not(.du-head) [data-du-tog]');
check('a branch opens', await until('document.querySelectorAll("#du-body [aria-expanded=\\"true\\"]").length === 1'));
await shot('usage-tree');
await page.click('[data-du-sort="name"]');
await wait(200);
check('sorting by name', (await text('#du-body .du-row:not(.du-head)')).startsWith('system'), await text('#du-body .du-row:not(.du-head)'));
await page.evaluate('CC.closeWin(); CC.run("diskUsage", "/mnt/user/backups")');
await wait(800);
await page.click('#du-view [data-v="tree"]');
await wait(300);
await page.click('#du-body .du-row:not(.du-head) >> nth=0');
await page.click('#du-body .du-row:not(.du-head) >> nth=1', { modifiers: ['Control'] });
check('two picked', /^2 ausgewählt/.test(await text('.bk-du .win-foot .start')), await text('.bk-du .win-foot .start'));
await page.click('#du-body .du-row.sel >> nth=0', { button: 'right' });
check('the row menu offers deleting both', /2 Objekte löschen/.test(await text('#menu')), await text('#menu'));
await page.keyboard.press('Escape');
await page.focus('#du-body');
await page.keyboard.press('Delete');
await wait(300);
await shot('usage-delete');
await page.locator('.scrim').last().locator('.win-foot .btn.primary').click();
check('the deleted rows leave the tree', await until('!/^dokumente|^vm-win11/.test(document.querySelector("#du-body .du-row:not(.du-head)")?.innerText || "") && document.querySelectorAll("#du-body .du-row:not(.du-head)").length > 0'), await text('#du-body'));
await page.click('#du-view [data-v="types"]');
await wait(300);
await shot('usage-types');
await page.click('#du-body [data-du-kind] >> nth=0');
await wait(500);
check('a type shows its largest files', /^Nur /.test(await text('.du-kind')) && (await page.locator('#du-body .du-row').count()) > 0, await text('.du-kind'));
await page.click('[data-du-allkinds]');
await wait(300);
await shot('usage-largest');
await closeAll();

check('no page errors', !errors.length, errors);
await browser.close();
console.log(failed ? `${failed} checks failed` : 'all checks passed');
process.exitCode = failed ? 1 : 0;
