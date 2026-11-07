// node tests/windows/zoom.ts [app-url] [shots-dir]: zooms the left panel with Ctrl and the wheel in the app running
// with `VITE_E2E=1 vite --mode fixture`, from the details up to the largest gallery and back, and checks that the right
// panel stays as it was, that a pinch moves in small steps, that a plain wheel only scrolls, and that a reload keeps
// the size. Playwright comes from the prototype's own install.
import { mkdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..', '..', '..');
const { chromium } = createRequire(join(root, 'prototype', 'package.json'))('playwright') as typeof import('playwright');
const app = process.argv[2] || 'http://localhost:5194/';
const shots = resolve(process.argv[3] || join(root, 'testing', 'out', 'zoom'));
mkdirSync(shots, { recursive: true });

const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1600, height: 950 }, locale: 'de-AT' });
const page = await context.newPage();
const errors: string[] = [];
page.on('pageerror', (e: Error) => errors.push(String(e).slice(0, 300)));
let failed = 0;
const check = (what: string, ok: unknown, seen?: unknown) => {
  if (!ok) failed++;
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${what}${ok ? '' : `: ${JSON.stringify(seen)}`}`);
};
const wait = (ms = 400) => page.waitForTimeout(ms);
const until = (fn: string, timeout = 20000) => page.waitForFunction(fn, null, { timeout }).then(() => true, () => false);

interface Shown { view: string; tile: string; zoomed: boolean; top: number; cards: number; big: number }
const shown = (panel: number) => page.evaluate((i) => {
  const l = document.querySelectorAll<HTMLElement>('.panel .list')[i]!;
  return {
    view: [...l.classList].find((c) => c.startsWith('view-'))!.slice(5),
    tile: l.style.getPropertyValue('--zoom-tile'),
    zoomed: l.classList.contains('zoomed'),
    top: l.scrollTop,
    cards: l.querySelector<HTMLElement>('.gcard')?.getBoundingClientRect().width ?? 0,
    big: l.querySelectorAll('img[src*="size=large"]').length,
  };
}, panel) as Promise<Shown>;
const overList = async (panel: number) => {
  const box = (await page.locator('.panel .list').nth(panel).boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + 120);
};
// Wheel events that reached the window with the browser's own handling taken away.
const taken = () => page.evaluate('window.zoomTaken.splice(0)') as Promise<boolean[]>;
async function ctrlWheel(dy: number, times = 1) {
  await page.keyboard.down('Control');
  for (let k = 0; k < times; k++) {
    await page.mouse.wheel(0, dy);
    await wait(30);
  }
  await page.keyboard.up('Control');
  await wait(350);
}

await page.goto(app);
check('the shim is ready', await until('document.documentElement.dataset.e2e === "ready"', 120000));
await page.evaluate(() => {
  const w = window as never as { zoomTaken: boolean[]; Files: { go(p: string, i: number): void } };
  w.zoomTaken = [];
  addEventListener('wheel', (e) => { if (e.ctrlKey) w.zoomTaken.push(e.defaultPrevented); });
  w.Files.go('/mnt/user/fotos/2026', 0);
  w.Files.go('/mnt/user/downloads', 1);
});
await until('document.querySelectorAll(".panel .list")[0].querySelectorAll(".row").length > 20');
await wait(600);
const right = await shown(1);
check('both panels start in the details', (await shown(0)).view === 'details' && right.view === 'details', [await shown(0), right]);

await overList(0);
await ctrlWheel(-100);
check('a notch up turns the details into the list', (await shown(0)).view === 'list', await shown(0));
check('the browser does not zoom the page over the files', (await taken()).every(Boolean));
await ctrlWheel(-100);
let left = await shown(0);
check('the next notch shows the smallest tiles', left.view === 'tiles' && left.tile === '96px' && left.zoomed, left);
check('the cards are as wide as the zoom says', Math.abs(left.cards - 96) < 1, left.cards);
await ctrlWheel(-100);
check('each notch grows the tiles a little', (await shown(0)).tile === '112px', await shown(0));
check('the view menu follows the zoom', await page.evaluate(`CC.actions['view-tiles'].checked() && !CC.actions['view-details'].checked()`));

await ctrlWheel(-100, 5);
check('the tiles grow into large ones', (await shown(0)).view === 'large', await shown(0));
await page.screenshot({ path: join(shots, 'zoom-large.png') });
check('small cards still ask for the small pictures', (await shown(0)).big === 0, await shown(0));
await ctrlWheel(-100, 30);
left = await shown(0);
check('the large ones turn into the gallery and stop at its largest', left.view === 'gallery' && left.tile === '400px', left);
check('a large card asks for the large pictures', left.big > 0, left);
await page.screenshot({ path: join(shots, 'zoom-gallery.png') });

const now = await shown(1);
check('the right panel stays as it was', now.view === right.view && now.tile === right.tile && now.top === right.top, [right, now]);

// A trackpad pinch sends many small steps with Ctrl held.
await ctrlWheel(10, 4);
const near = (tile: string, px: number) => Math.abs(parseFloat(tile) - px) < 0.01;
check('a pinch shrinks the gallery smoothly', near((await shown(0)).tile, 393.6), await shown(0));

const before = await shown(0);
await page.mouse.wheel(0, 300);
await wait(400);
left = await shown(0);
check('a wheel without Ctrl only scrolls', left.view === before.view && left.tile === before.tile && left.top > before.top, [before, left]);

check('every zoom over the files keeps the browser out of it', (await taken()).every(Boolean));
const side = (await page.locator('#tree').boundingBox())!;
await page.mouse.move(side.x + side.width / 2, side.y + 40);
await ctrlWheel(-100);
const outside = await taken();
check('Ctrl and the wheel outside the files leave the page zoom to the browser', outside.length > 0 && outside.every((x) => !x), outside);

await wait(1200);
await page.reload();
check('the shim is ready again', await until('document.documentElement.dataset.e2e === "ready"', 120000));
await page.evaluate(`Files.go('/mnt/user/fotos/2026', 0)`);
await until('document.querySelectorAll(".panel .list")[0]?.classList.contains("view-gallery")');
check('a reload keeps the zoomed view and size', near((await shown(0)).tile, 393.6), await shown(0));

await overList(0);
await ctrlWheel(100, 30);
check('the wheel walks back down to the most compact rows', (await shown(0)).view === 'compact', await shown(0));
await page.evaluate(`CC.run('view-large')`);
await wait(300);
left = await shown(0);
check('a view picked from the menu shows at its own size again', left.view === 'large' && !left.zoomed, left);

check('no page errors', !errors.length, errors);
await browser.close();
console.log(failed ? `${failed} checks failed` : 'all checks passed');
process.exit(failed ? 1 : 0);
