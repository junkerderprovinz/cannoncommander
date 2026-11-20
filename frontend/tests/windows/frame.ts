// node tests/windows/frame.ts [app-url] [shots-dir]: the app running with `vite --mode fixture`, told it is the desktop
// app on Windows, which puts its own window buttons at the end of the head. Checks at the narrowest window and wider
// ones that they stand in the head's row with its usual gap and that nothing in the head lies under them. Playwright
// comes from the prototype's install.
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
  await page.waitForSelector('.head .window-controls .glim-window-square', { timeout: 30000 });
  await page.waitForTimeout(500);
  const seen = await page.evaluate(() => {
    const box = (e: Element) => e.getBoundingClientRect();
    const squares = [...document.querySelectorAll('.head .window-controls .glim-window-square')];
    const boxes = squares.map(box), controls = document.querySelector('.head .window-controls')!;
    const head = document.querySelector('.head')!, hb = box(head), avatar = box(document.querySelector('.head .avatar')!);
    const meets = (a: DOMRect, b: DOMRect) => a.left < b.right && b.left < a.right && a.top < b.bottom && b.top < a.bottom;
    const under = [...document.querySelectorAll('.head *')].filter((e) => {
      if (e.contains(controls) || controls.contains(e)) return false;
      const r = box(e);
      return r.width > 0 && r.height > 0 && boxes.some((b) => meets(r, b));
    }).map((e) => `${e.tagName.toLowerCase()}.${String((e as HTMLElement).className).split(' ')[0]}`);
    const mid = (r: DOMRect) => r.top + r.height / 2;
    return {
      under, buttons: boxes.length,
      centred: boxes.every((b) => Math.abs(mid(b) - mid(hb)) <= 1),
      height: boxes.map((b) => Math.round(b.height)), avatarHeight: Math.round(avatar.height),
      gap: Math.round((box(controls).left - avatar.right) * 10) / 10, headGap: parseFloat(getComputedStyle(head).columnGap),
      inside: boxes.every((b) => b.right <= hb.right && b.top >= hb.top && b.bottom <= hb.bottom),
      bar: !!document.querySelector('.glim-window-bar'),
    };
  });
  check(`${width}px: three window buttons at the end of the head`, seen.buttons === 3 && !seen.bar, seen);
  check(`${width}px: nothing in the head under the window buttons`, seen.under.length === 0, seen.under);
  check(`${width}px: as tall as the head's buttons and centred in its row`, seen.centred && seen.height.every((h) => h === seen.avatarHeight), seen);
  check(`${width}px: the head's own gap before them, and inside the head`, Math.abs(seen.gap - seen.headGap) <= 1 && seen.inside, seen);
  await page.screenshot({ path: join(shots, `frame-${width}.png`), clip: { x: 0, y: 0, width, height: 120 } });
  await page.close();
}
await browser.close();
console.log(failed ? `${failed} failed` : 'all ok');
process.exit(failed ? 1 : 0);
