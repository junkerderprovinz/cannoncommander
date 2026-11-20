// node tests/windows/frame.ts [app-url] [shots-dir]: the app running with `vite --mode fixture`, told it is the desktop
// app on Windows, which draws its own title bar. Checks at the narrowest window and wider ones that nothing in the head
// lies under the window buttons and that the head starts below the bar. Playwright comes from the prototype's install.
import { mkdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..', '..', '..');
const { chromium } = createRequire(join(root, 'prototype', 'package.json'))('playwright') as typeof import('playwright');
const app = process.argv[2] || 'http://localhost:5193/';
const shots = resolve(process.argv[3] || join(root, 'testing', 'out', 'frame'));
mkdirSync(shots, { recursive: true });

// The desktop app's smallest window, from desktop/main.go, and two wider ones.
const WIDTHS = [800, 1100, 1600];
let failed = 0;
const check = (what: string, ok: unknown, seen?: unknown) => {
  if (!ok) failed++;
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${what}${ok ? '' : `: ${JSON.stringify(seen)}`}`);
};

const browser = await chromium.launch();
for (const width of WIDTHS) {
  const page = await (await browser.newContext({ viewport: { width, height: 700 }, locale: 'de-AT' })).newPage();
  await page.route('**/api/v1/system', async (route) => {
    const got = await route.fetch();
    await route.fulfill({ response: got, json: { ...(await got.json()), deployment: 'desktop', os: 'windows' } });
  });
  await page.goto(app);
  await page.waitForSelector('.glim-window-bar .glim-window-square', { state: 'attached', timeout: 30000 });
  await page.waitForSelector('.head', { timeout: 30000 });
  await page.waitForTimeout(500);
  const seen = await page.evaluate(() => {
    const box = (e: Element) => e.getBoundingClientRect();
    // The round squares are what shows, also when a bar without height lets them hang over the page.
    const buttons = [...document.querySelectorAll('.glim-window-bar .glim-window-square')].map(box);
    const bar = box(document.querySelector('.glim-window-bar')!), head = box(document.querySelector('.head')!);
    const meets = (a: DOMRect, b: DOMRect) => a.left < b.right && b.left < a.right && a.top < b.bottom && b.top < a.bottom;
    const under = [...document.querySelectorAll('.head, .head *')].filter((e) => {
      const r = box(e);
      return r.width > 0 && r.height > 0 && buttons.some((b) => meets(r, b));
    }).map((e) => `${e.tagName.toLowerCase()}.${String((e as HTMLElement).className).split(' ')[0]}`);
    const inside = buttons.every((b) => b.top >= bar.top && b.bottom <= bar.bottom);
    return { under, inside, bar: { top: bar.top, bottom: bar.bottom, height: bar.height }, headTop: head.top, buttons: buttons.length };
  });
  check(`${width}px: three window buttons in the bar`, seen.buttons === 3, seen);
  check(`${width}px: nothing in the head under the window buttons`, seen.under.length === 0, seen.under);
  check(`${width}px: the window buttons sit inside the bar`, seen.inside, seen);
  check(`${width}px: the head starts below the bar`, seen.headTop >= seen.bar.bottom, seen);
  await page.screenshot({ path: join(shots, `frame-${width}.png`), clip: { x: 0, y: 0, width, height: 120 } });
  await page.close();
}
await browser.close();
console.log(failed ? `${failed} failed` : 'all ok');
process.exit(failed ? 1 : 0);
