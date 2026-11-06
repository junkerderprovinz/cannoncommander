// node tests/windows/files.ts [app-url] [shots-dir]: drives the rename, archive, checksum, property, clean up, fetch,
// share, download and ArrowLoop windows in the app running with `VITE_E2E=1 vite --mode fixture`, checks what each one
// does to the fixture's tree and keeps a screenshot of each. Playwright comes from the prototype's own install.
// It changes the tree as it goes, so it wants a fixture server of its own, started fresh.
import { mkdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..', '..', '..');
const { chromium } = createRequire(join(root, 'prototype', 'package.json'))('playwright') as typeof import('playwright');
const app = process.argv[2] || 'http://localhost:5193/';
const shots = resolve(process.argv[3] || join(root, 'testing', 'out', 'files'));
mkdirSync(shots, { recursive: true });

const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1600, height: 950 }, locale: 'de-AT', acceptDownloads: true });
const page = await context.newPage();
const errors: string[] = [];
page.on('pageerror', (e: Error) => errors.push(String(e).slice(0, 300)));
let failed = 0;
const check = (what: string, ok: unknown, seen?: unknown) => {
  if (!ok) failed++;
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${what}${ok ? '' : `: ${JSON.stringify(seen)}`}`);
};
const wait = (ms = 400) => page.waitForTimeout(ms);
// Windows fade in, so a picture waits until they stand still.
const shot = async (name: string) => { await wait(350); await page.screenshot({ path: join(shots, name + '.png') }); };
const until = (fn: string, timeout = 20000) => page.waitForFunction(fn, null, { timeout }).then(() => true, () => false);
const top = () => page.locator('.scrim').last();
const primary = () => top().locator('.win-foot .btn.primary').click();
const closeAll = () => page.evaluate('while (winStack.length) CC.closeWin(); closeMenu()');
const names = (panel = 0) => page.evaluate(`Files.entries(${panel}).map((k) => k.name)`) as Promise<string[]>;
const toasts = () => page.locator('.toast').allInnerTexts();
const api = (method: string, path: string, body?: unknown) => page.evaluate(([m, p, b]) => fetch('/api/v1' + p, { method: m as string, headers: { 'Content-Type': 'application/json' }, body: b ? JSON.stringify(b) : undefined }).then((r) => (r.status === 204 ? null : r.json())), [method, path, body] as const);
// Opens the folder in a panel and waits until it is read, then puts the cursor and the marks on the names.
async function at(path: string, picked: string[], panel = 0) {
  await page.evaluate(([p, i]) => { const F = (window as never as { Files: { active: number; go(p: string, i: number): void } }).Files; F.active = i as number; F.go(p as string, i as number); }, [path, panel] as const);
  await page.waitForFunction(([p, i, n]) => {
    const F = (window as never as { Files: { T(i: number): { path: string; marked: Set<string>; cursor: number }; entries(i: number): { name: string }[] } }).Files;
    const list = F.entries(i as number), want = n as string[];
    if (F.T(i as number).path !== p || (want.length && list.findIndex((e) => e.name === want[0]) < 0)) return false;
    F.T(i as number).cursor = Math.max(0, list.findIndex((e) => e.name === want[0]));
    F.T(i as number).marked = new Set(want.length > 1 ? want : []);
    return true;
  }, [path, panel, picked] as const);
}
const runAction = (id: string) => page.evaluate((a) => (window as never as { CC: { run(id: string): void } }).CC.run(a), id);

await page.goto(app);
check('the shim is ready', await until('document.documentElement.dataset.e2e === "ready"', 120000));
await wait(600);

// Mehrfach umbenennen: the preview comes from the server's dry run, metadata tokens included.
await at('/mnt/user/fotos/Urlaub', []);
await page.evaluate('Files.T().marked = new Set(Files.entries().filter((k) => !k.dir).slice(0, 4).map((k) => k.name))');
await page.keyboard.press('Control+m');
await wait(500);
await page.fill('#m-name', 'Urlaub_[C3]');
await page.click('#m-case');
await page.locator('#menu [role=option]', { hasText: 'alles klein' }).first().click();
await wait(600);
check('the rename preview counts the changes', /4 Namen ändern sich/.test(await page.textContent('#m-count') || ''), await page.textContent('#m-count'));
check('the preview shows the new names', (await page.locator('#m-list .bk-mrow .new').allInnerTexts())[0] === 'urlaub_001.jpg', await page.locator('#m-list .bk-mrow .new').allInnerTexts());
await shot('rename');
await page.click('[data-bk-tok="[K]"]');
await wait(600);
check('a metadata token fills in the camera', /urlaub_001(pixel 9|iphone 16)/i.test((await page.locator('#m-list .bk-mrow .new').allInnerTexts())[0] || ''), await page.locator('#m-list .bk-mrow .new').allInnerTexts());
await page.fill('#m-name', 'Urlaub_[C3]');
await wait(500);
await primary();
check('the photos are renamed', await until('Files.entries().some((k) => k.name === "urlaub_001.jpg")'));
check('the rename offers to undo it', (await toasts()).some((x) => /4 Namen geändert/.test(x)), await toasts());
await shot('rename-done');

// Im Editor umbenennen: one name per line, a missing line is called out.
await page.evaluate('Files.T().marked = new Set(["urlaub_001.jpg", "urlaub_002.jpg"])');
await runAction('renameEditor');
await wait(400);
await page.fill('.re-ta', 'strand.jpg');
await wait(200);
check('a missing line is called out', /nur 1 Zeilen für 2 Namen/.test(await page.textContent('#re-warn') || ''), await page.textContent('#re-warn'));
await page.fill('.re-ta', 'strand.jpg\nmeer.jpg');
await wait(200);
await shot('rename-editor');
await primary();
check('the editor renames both', await until('Files.entries().some((k) => k.name === "strand.jpg") && Files.entries().some((k) => k.name === "meer.jpg")'));
await closeAll();

// Nach Muster markieren.
await at('/mnt/user/fotos/2026', []);
await page.keyboard.press('+');
await wait(300);
await shot('pattern');
await primary();
check('the pattern marks the pictures', ((await page.evaluate('Files.T().marked.size')) as number) > 0);
await page.evaluate('Files.T().marked = new Set()');

// Verpacken and Entpacken run as jobs; the new archive lands in the other panel.
await at('/mnt/user/backups', [], 1);
await at('/mnt/user/dokumente', ['Mietvertrag.pdf', 'notizen.txt']);
await page.keyboard.press('Alt+p');
await wait(400);
check('the pack window aims at the other panel', (await page.inputValue('#k-dest')) === '/mnt/user/backups', await page.inputValue('#k-dest'));
await page.click('#k-fmt [data-v="tar.zst"]');
await wait(150);
check('tar.zst has no password', await page.isDisabled('#k-pw'));
await page.click('#k-fmt [data-v="zip"]');
await shot('pack');
await primary();
check('the archive lands in backups', await until('Files.entries(1).some((k) => k.name === "dokumente.zip")'));
await at('/mnt/user/downloads', ['Fotos-Urlaub-2026.zip']);
await page.keyboard.press('Alt+u');
await wait(500);
check('unpack says how much it makes', /Ergibt/.test(await top().locator('.win-foot .start').innerText()), await top().locator('.win-foot .start').innerText());
await shot('unpack');
await page.fill('#x-dest', '/mnt/user/downloads');
await wait(300);
await primary();
check('the archive unpacks into a folder of its own', await until('Files.entries().some((k) => k.name === "Fotos-Urlaub-2026" && k.dir)'));

// Archiv testen marks the damaged backup in the list.
await at('/mnt/user/backups', ['appdata-2026-10-24.tar.zst']);
await runAction('testArchive');
check('the test finds the damage', await until('/defekt/.test(document.querySelector("#at-sum")?.textContent || "")'));
await shot('test-archive');
await closeAll();
await wait(300);
check('the damaged archive is red in the list', await page.locator('.row .bk-damaged').count() > 0);
await shot('test-archive-list');

// Prüfsummen erstellen, then prüfen with the list the window saved.
await at('/mnt/user/isos', ['Win11_24H2_German_x64.iso', 'debian-13.1.0-amd64-netinst.iso', 'ubuntu-24.04.3-desktop-amd64.iso']);
await runAction('checksum');
await wait(300);
await shot('sums-running');
check('every file gets its checksum', await until('document.querySelectorAll("#c-list .bk-hash span[title]").length === 3'));
await shot('sums');
await page.fill('#c-name', 'neu.sha256');
await primary();
check('the list is saved beside the files', await until('Files.entries().some((k) => k.name === "neu.sha256")'));
await closeAll();
await at('/mnt/user/isos', ['checksums.sha256']);
await runAction('checksumVerify');
check('verify finds the one that differs', await until('document.querySelectorAll("#v-list .pill.fault").length === 1'));
await shot('verify');
await closeAll();

// Datei teilen and Zusammenfügen.
await at('/mnt/user/downloads', ['Big.Buck.Bunny.2008.1080p.mkv']);
await runAction('split');
await wait(300);
await shot('split');
await closeAll();
await at('/mnt/user/downloads/unfertig', ['Fotos-2019.7z.002']);
await runAction('combine');
await wait(300);
check('combine finds all three parts', (await top().locator('.tbl tr').count()) === 3, await top().locator('.tbl tr').count());
await shot('combine');
await primary();
check('the parts are combined', await until('Files.entries().some((k) => k.name === "Fotos-2019.7z")'));

// Symbolische Verknüpfung and Eigenschaften.
await at('/mnt/user/downloads', [], 1);
await at('/mnt/user/dokumente', ['Einkaufsliste.md']);
await runAction('symlink');
await wait(300);
await page.fill('#l-name', 'einkauf.md');
await shot('symlink');
await primary();
check('the link lies in downloads', await until('Files.entries(1).some((k) => k.name === "einkauf.md" && k.link === "/mnt/user/dokumente/Einkaufsliste.md")'));
await at('/mnt/user/fotos', ['2026']);
await page.keyboard.press('Alt+Enter');
check('properties count the folder', await until('/Dateien/.test(document.querySelector(".bk-props .bk-val.glim-num")?.textContent || "")'));
await shot('props');
await page.click('[data-bk-perm="2:2"]');
check('a ticked box changes the number', (await page.inputValue('#r-oct')) === '777', await page.inputValue('#r-oct'));
await primary();
await wait(600);
check('the rights are saved', ((await api('GET', '/fs/list?path=/mnt/user/fotos')) as { items: { name: string; mode: string }[] }).items.find((k) => k.name === '2026')?.mode === '0777');
await at('/mnt/user/dokumente', ['Mietvertrag.pdf', 'docker-compose.yml']);
await page.keyboard.press('Alt+Enter');
await wait(500);
await shot('props-many');
await closeAll();

// Aufräumen finds the leftovers in unfertig.
await at('/mnt/user/downloads/unfertig', []);
await runAction('cleanup');
check('clean up finds something', await until('document.querySelectorAll("#cl-list .cl-row").length > 3'));
await page.click('[data-cl-group="name"]');
await wait(200);
await shot('cleanup');
await primary();
check('bad names get a better one', await until('!Files.entries().some((k) => k.name === "Notiz: wichtig?.txt")'));
await closeAll();

// Von Adresse laden.
await at('/mnt/user/downloads', []);
await runAction('fetchUrl');
await wait(300);
await page.fill('#fu-url', 'https://example.org/files/debian-netinst.iso');
await wait(100);
check('the name comes from the address', (await page.inputValue('#fu-name')) === 'debian-netinst.iso');
await shot('fetch');
await primary();
check('the file arrives', await until('Files.entries().some((k) => k.name === "debian-netinst.iso")', 30000));

// Freigeben makes a link with its QR code, the Sammelmappe gathers from two folders.
await at('/mnt/user/dokumente', ['Mietvertrag.pdf']);
await runAction('share');
await wait(400);
await page.click('#cs-pw');
await page.fill('#cs-pass', 'geheim123');
await shot('share');
await primary();
check('the link window shows a QR code', await until('!!document.querySelector(".cs-qr svg")'));
await shot('share-done');
await closeAll();
const links = (await api('GET', '/shares')) as { paths: string[]; password: boolean }[];
check('the server keeps the link', links.some((s) => s.paths[0] === '/mnt/user/dokumente/Mietvertrag.pdf' && s.password), links);

// The message for the visitors can be changed later in Einstellungen, Freigaben.
await page.locator('[aria-label="Einstellungen"]').first().click();
await page.getByText('Freigaben', { exact: true }).first().click();
await wait(400);
await page.locator('.a-list').getByText('Mietvertrag.pdf', { exact: true }).first().click();
await wait(400);
check('the share window offers the message', (await top().locator('#a-shmsg').count()) === 1);
await top().locator('#a-shmsg').fill('  Hier ist der Vertrag zum Unterschreiben.  ');
await shot('share-message');
await primary();
await wait(400);
const edited = (await api('GET', '/shares')) as { paths: string[]; message: string }[];
check('the server keeps the new message', edited.some((s) => s.paths[0] === '/mnt/user/dokumente/Mietvertrag.pdf' && s.message === 'Hier ist der Vertrag zum Unterschreiben.'), edited);
await closeAll();
await runAction('collect');
await at('/mnt/user/fotos/2026', ['IMG_0203.jpg']);
await runAction('collect');
check('the sidebar counts the collection', (await page.locator('[data-run="collection"] .meta').innerText()) === '2');
await runAction('collection');
await wait(300);
await shot('collection');
await primary();
await wait(300);
await page.fill('#cs-title', 'Für Anna');
await primary();
check('the collection link shows what the other side sees', await until('!!document.querySelector(".cs-visitor")'));
await shot('collection-done');
await closeAll();

// Herunterladen: one file straight away, a folder packed by the server.
await at('/mnt/user/dokumente', ['notizen.txt']);
const one = page.waitForEvent('download');
await runAction('download');
check('one file downloads as it is', (await one).suggestedFilename() === 'notizen.txt');
await at('/mnt/user', ['dokumente']);
await runAction('download');
await wait(300);
await shot('download');
const packed = page.waitForEvent('download');
await primary();
check('a folder downloads as a zip', (await packed).suggestedFilename() === 'dokumente.zip');

// In ArrowLoop planen, as the sync window opens it.
await page.evaluate('CC.setPref("dual", true)');
await page.evaluate('CC.run("arrowLoop", "/mnt/user/dokumente", "/mnt/user/backups/dokumente", { direction: "right" })');
await wait(300);
await shot('arrowloop');
await primary();
await wait(400);
check('the job is handed over', (await toasts()).some((x) => /in ArrowLoop geplant/.test(x)), await toasts());

// The row menu reaches every tool.
await at('/mnt/user/downloads', ['Fotos-Urlaub-2026.zip']);
await page.locator('.panel.active .row.cursor').click({ button: 'right' });
await wait(300);
const rowMenu = (await page.locator('#menu').innerText()).replace(/\s+/g, ' ');
for (const label of ['Freigeben', 'In die Sammelmappe', 'Herunterladen', 'Verpacken', 'Entpacken', 'Archiv testen', 'Prüfsummen erstellen', 'Symbolische Verknüpfung', 'Eigenschaften und Rechte', 'Pfad kopieren']) {
  check(`the row menu has ${label}`, rowMenu.includes(label), rowMenu);
}
await shot('row-menu');

// A typed path that is not there keeps the field, which shakes, and the panel stays where it was.
await closeAll();
await at('/mnt/user/dokumente', []);
await runAction('gotoPath');
await wait(200);
await page.fill('.panel.active .pathedit', '/mnt/user/dokumente/gibtsnicht');
await page.keyboard.press('Enter');
await wait(300);
check('a typed path that is not there stays in the field and shakes', await page.locator('.panel.active .pathedit.glim-shake').count() === 1);
check('it names the folder', (await toasts()).some((x) => x.includes('Den Ordner /mnt/user/dokumente/gibtsnicht gibt es nicht')), await toasts());
check('the panel stays where it was', await page.evaluate('Files.T().path') === '/mnt/user/dokumente');
await page.keyboard.press('Escape');

// A folder the index has not measured yet shows a quiet mark instead of 0 B, sorts after the measured ones, and its
// size comes in on its own once the index has it.
await api('PUT', '/fixture/unmeasured', { paths: ['/mnt/user/dokumente', '/mnt/user/filme'] });
await at('/mnt/user', []);
const unmeasured = 'Files.entries().filter((k) => k.sizeUnknown).map((k) => k.name).sort().join()';
check('the folders still to measure are marked', await until(`${unmeasured} === "dokumente,filme"`), await page.evaluate(unmeasured));
await page.evaluate('Object.assign(Files.T(), { sort: "size", dir: -1 })');
await wait(300);
const folders = await page.evaluate('Files.entries().filter((k) => k.dir).map((k) => k.name)') as string[];
check('they sort after the measured folders', folders.slice(-2).sort().join() === 'dokumente,filme', folders);
const mark = page.locator('.panel.active .row', { hasText: 'dokumente' }).locator('[data-unmeasured]');
check('the size column shows a quiet mark with a tip', await mark.count() === 1 && /noch nicht gemessen/.test(await mark.getAttribute('data-tip') || ''), await mark.count());
await shot('unmeasured');
await page.evaluate('Files.T().marked = new Set(["dokumente", "fotos"])');
await wait(200);
check('the foot says the sum is not the whole', /2 markiert, mindestens /.test(await page.locator('.panel.active .lfoot').innerText()), await page.locator('.panel.active .lfoot').innerText());
await page.evaluate('Object.assign(Files.T(), { sort: "name", dir: 1, marked: new Set() })');
await api('PUT', '/fixture/unmeasured', { paths: [] });
check('the size comes in without a refresh', await until(`${unmeasured} === ""`, 45000), await page.evaluate(unmeasured));

check('no page errors', !errors.length, errors);

// A start folder that is gone opens the nearest folder above it, says so, and its tree is asked for at most once.
const late = await browser.newContext({ viewport: { width: 1600, height: 950 }, locale: 'de-AT' });
await late.addInitScript(() => localStorage.setItem('cannoncommander-prefs', JSON.stringify({ startFolder: '/mnt/user/downloads/weg/tiefer' })));
const start = await late.newPage();
const trees: string[] = [];
start.on('request', (r) => { if (r.url().includes('/fs/tree') && r.url().includes('weg')) trees.push(r.url()); });
start.on('pageerror', (e: Error) => errors.push(String(e).slice(0, 300)));
await start.goto(app);
const said = await start.waitForFunction(() => [...document.querySelectorAll('.toast')].some((t) => t.textContent?.includes('Den Ordner /mnt/user/downloads/weg/tiefer gibt es nicht, darum bist du jetzt in /mnt/user/downloads')), null, { timeout: 60000 }).then(() => true, () => false);
check('a missing start folder says where the panel went', said, await start.locator('.toast').allInnerTexts());
check('the panel is in the nearest folder above it', await start.evaluate(() => location.hash) === '#/mnt/user/downloads', await start.evaluate(() => location.hash));
await start.screenshot({ path: join(shots, 'start-missing.png') });
await start.waitForFunction('document.documentElement.dataset.e2e === "ready"', null, { timeout: 120000 }).catch(() => {});
check('the missing folder is asked for at most once', trees.length <= 1, trees);
check('no page errors on the late start', !errors.length, errors);
await late.close();
await browser.close();
console.log(failed ? `${failed} checks failed` : 'all checks passed');
process.exit(failed ? 1 : 0);
