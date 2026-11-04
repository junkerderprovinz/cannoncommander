// node frontend/tests/parity/run.ts [--a page] [--b page] [--steps r13-unraid,core] [--jobs 4] [--out dir] [--update]
// Runs every step file of the prototype against two pages, the prototype (a) and the app (b), with the same clock
// and the same random numbers, then compares what the steps print and what the screenshots show. A page is a
// path to an HTML file or a URL; b defaults to PARITY_APP_URL and, without it, to the prototype itself.
// Differences the app may have are listed per step file in allow.json, each with its reason.
import { execFileSync, fork } from 'node:child_process';
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { basename, join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { format, parseArgs } from 'node:util';
import type { Browser, Page } from 'playwright';
import { decode, diff, encode } from './png.ts';

const here = import.meta.dirname;
const repo = resolve(here, '..', '..', '..');
const prototype = join(repo, 'prototype');
const tools = join(prototype, 'tools');
const fromPrototype = createRequire(join(prototype, 'package.json'));

const { values: opt } = parseArgs({
  options: {
    a: { type: 'string', default: join(prototype, 'dist', 'cannoncommander.html') },
    b: { type: 'string', default: process.env.PARITY_APP_URL || '' },
    steps: { type: 'string', default: '' },
    jobs: { type: 'string', default: '4' },
    out: { type: 'string', default: join(repo, 'testing', 'out', 'parity') },
    update: { type: 'boolean', default: false },
    worker: { type: 'string' },
  },
});
opt.b ||= opt.a;

// The fixture on disk was made at this moment, so both pages have to agree with it on what "today" is.
const NOW: number = JSON.parse(readFileSync(join(repo, 'testing', 'fixture', 'prototype.json'), 'utf8')).now;
const VIEWPORT = { width: 1600, height: 950 };
const SIDE_TIMEOUT = 10 * 60_000;

interface Rule { pattern: string; why: string }
interface ShotRule { pixels: number; why: string }
// page: a prototype page of its own that the app has no counterpart for, run on both sides.
// stale: why the step file fails on the prototype as well, until it is brought up to date.
interface Allow { lines?: Rule[]; shots?: Record<string, ShotRule>; page?: string; stale?: string }
interface Side { lines: string[]; shots: string[]; errors: string[]; ms: number }
interface Result { steps: string; a: Side; b: Side; problems: string[]; allowed: string[] }

const allowAll: Record<string, Allow> = JSON.parse(readFileSync(join(here, 'allow.json'), 'utf8'));
const allowFor = (steps: string): Allow => {
  const own = allowAll[steps] || {}, any = allowAll['*'] || {};
  return { ...own, lines: [...(any.lines || []), ...(own.lines || [])], shots: { ...any.shots, ...own.shots } };
};

const urlOf = (page: string) => (/^https?:/.test(page) ? page : pathToFileURL(resolve(page)).href);

// The same sequence of random numbers on both pages, again after every reload.
const seedRandom = () => {
  let s = 0x2545f491;
  Math.random = () => {
    s = (s + 0x6d2b79f5) | 0;
    let t = Math.imul(s ^ (s >>> 15), 1 | s);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
};

// The page's clock stands still and moves only while a step file waits, so jobs, toasts and timers reach the
// same point on every run no matter how fast the machine is. Waiting for a condition steps the clock until it holds.
// The prototype keeps everything in memory; the app asks its server, and an answer arrives in real time while the
// page clock stands still. A wait lets the answers in flight land first, so both pages reach the same point.
function inFlight(tab: Page) {
  let open = 0;
  tab.on('request', () => { open++; });
  tab.on('requestfinished', () => { open--; });
  tab.on('requestfailed', () => { open--; });
  const sleep = (ms: number) => new Promise((done) => setTimeout(done, ms));
  // A request the page has just sent takes a moment to be reported, so the count is read after a short pause.
  return async () => {
    await sleep(30);
    for (let waited = 0; open > 0 && waited < 5000; waited += 10) await sleep(10);
  };
}

function driven(tab: Page): Page {
  const settle = inFlight(tab);
  const until = async (test: () => Promise<unknown>, timeout = 30_000, what = 'condition') => {
    for (let waited = 0; ; waited += 50) {
      const value = await test();
      if (value) return value;
      if (waited >= timeout) throw new Error(`${what} not met after ${timeout} ms of page time`);
      await tab.clock.runFor(50);
    }
  };
  // An action waits for its element to show up and hold still, both of which can take page time, so the clock moves
  // on in steps while the element is missing and between tries on one that keeps moving. The number is where the
  // method takes its options.
  const OPTIONS_AT: Record<string, number> = {
    click: 1, dblclick: 1, hover: 1, focus: 1, textContent: 1, inputValue: 1, isEnabled: 1,
    getAttribute: 2, fill: 2, selectOption: 2, setInputFiles: 2, dragAndDrop: 2, dispatchEvent: 3,
  };
  const acting = (name: string) => async (...args: unknown[]) => {
    const given = (args[OPTIONS_AT[name]!] as { timeout?: number } | undefined) || {};
    const target = tab.locator(args[0] as string).first();
    // textContent and the like also read hidden elements, so for them being there is enough.
    const needsSight = !['textContent', 'getAttribute', 'dispatchEvent', 'setInputFiles'].includes(name);
    let there = false;
    for (let waited = 0; !(there = !!(needsSight ? await target.isVisible() : await target.count())); waited += 250) {
      if (waited >= (given.timeout ?? 30_000)) break;
      await tab.clock.runFor(250);
    }
    const fn = (tab as unknown as Record<string, (...a: unknown[]) => Promise<unknown>>)[name]!.bind(tab);
    for (let waited = 0; ; waited += 250) {
      const tryArgs = [...args];
      // An element that never showed up fails at once with Playwright's own message.
      tryArgs[OPTIONS_AT[name]!] = { ...given, timeout: there ? 2000 : 1 };
      try {
        return await fn(...tryArgs);
      } catch (e) {
        if ((e as Error).name !== 'TimeoutError' || !there || waited >= (given.timeout ?? 30_000)) throw e;
      }
      await tab.clock.runFor(250);
    }
  };
  // A script that returns at once settles long before the first pause; one that waits for a page timer gets the
  // clock moved on until it settles.
  const sleep = (ms: number) => new Promise((done) => setTimeout(done, ms));
  const evaluate = async (fn: unknown, arg?: unknown) => {
    let settled = false;
    const result = tab.evaluate(fn as never, arg).finally(() => { settled = true; });
    result.catch(() => {});
    await Promise.race([result, sleep(200)]);
    while (!settled) {
      await tab.clock.runFor(50);
      await Promise.race([result, sleep(30)]);
    }
    return result;
  };
  const own: Partial<Page> = {
    ...Object.fromEntries(Object.keys(OPTIONS_AT).map((name) => [name, acting(name)])),
    evaluate: evaluate as never,
    waitForTimeout: async (ms: number) => {
      await settle();
      await tab.clock.runFor(ms);
      await settle();
    },
    waitForFunction: ((fn: () => unknown, arg?: unknown, o?: { timeout?: number }) =>
      until(() => tab.evaluate(fn as never, arg), o?.timeout, 'waitForFunction')) as never,
    waitForSelector: ((selector: string, o?: { state?: string; timeout?: number }) =>
      until(async () => {
        const visible = await tab.locator(selector).first().isVisible();
        const there = (await tab.locator(selector).count()) > 0;
        const state = o?.state || 'visible';
        return state === 'visible' ? visible : state === 'hidden' ? !visible : state === 'detached' ? !there : there;
      }, o?.timeout, selector)) as never,
  };
  return new Proxy(tab, {
    get: (t, key) => {
      if (key in own) return own[key as keyof Page];
      const v = Reflect.get(t, key);
      return typeof v === 'function' ? v.bind(t) : v;
    },
  });
}

// CSS transitions, animations and smooth scrolling run on the browser's own clock, which the page clock does not
// reach, so both pages jump straight to the end of every one.
const stillMotion = () => {
  document.addEventListener('DOMContentLoaded', () => {
    const style = document.createElement('style');
    style.textContent = '*, *::before, *::after { transition-duration: 0s !important; transition-delay: 0s !important; animation-duration: 0s !important; animation-delay: 0s !important; scroll-behavior: auto !important; }';
    document.head.append(style);
  });
  const animate = Element.prototype.animate;
  // The end of an animation comes with the next real frame, so its handler runs on the page clock instead.
  Element.prototype.animate = function (this: Element, frames, options) {
    const timing = typeof options === 'number' ? {} : options || {};
    const a = animate.call(this, frames, { ...timing, duration: 0, delay: 0, endDelay: 0 });
    Object.defineProperty(a, 'onfinish', {
      set(fn: ((e: Event) => void) | null) {
        if (fn) setTimeout(() => fn.call(a, new Event('finish')), 0);
      },
    });
    return a;
  };
  const instant = <T extends unknown[]>(fn: (...a: T) => void) => function (this: Element, ...a: T) {
    const o = a[0];
    if (o && typeof o === 'object') a[0] = { ...o, behavior: 'instant' } as T[0];
    return fn.apply(this, a);
  };
  for (const proto of [Element.prototype, window] as unknown as Record<string, (...a: unknown[]) => void>[]) {
    for (const name of ['scrollTo', 'scrollBy', 'scroll', 'scrollIntoView']) if (proto[name]) proto[name] = instant(proto[name]);
  }
  // A file handed in through setInputFiles carries the browser's real time; it gets the page time it arrived at.
  const real = Object.getOwnPropertyDescriptor(File.prototype, 'lastModified')!.get!;
  const seen = new WeakMap<File, number>();
  Object.defineProperty(File.prototype, 'lastModified', {
    get(this: File) {
      const t = real.call(this) as number;
      if (t <= Date.now()) return t;
      if (!seen.has(this)) seen.set(this, Date.now());
      return seen.get(this);
    },
  });
};

async function runSide(browser: Browser, file: string, page: string, dir: string, preview: boolean): Promise<Side> {
  rmSync(dir, { recursive: true, force: true });
  mkdirSync(dir, { recursive: true });
  const context = await browser.newContext({ viewport: VIEWPORT, deviceScaleFactor: 1, locale: 'de-AT', timezoneId: 'Europe/Vienna' });
  const tab: Page = await context.newPage();
  const errors: string[] = [];
  tab.on('pageerror', (e) => errors.push(String(e).slice(0, 300)));
  tab.on('console', (m) => { if (m.type() === 'error') errors.push(m.text().slice(0, 300)); });
  await tab.clock.install({ time: NOW });
  await tab.clock.pauseAt(NOW + 1000);
  await tab.addInitScript(seedRandom);
  await tab.addInitScript(stillMotion);
  const shots: string[] = [];
  const shot = async (name: string) => {
    shots.push(name);
    await tab.screenshot({ path: join(dir, name + '.png'), animations: 'disabled', caret: 'hide' });
  };
  const lines: string[] = [];
  const log = console.log;
  const started = Date.now();
  let timer: NodeJS.Timeout | undefined;
  // Step files print their findings with console.log; those lines are what both pages have to agree on.
  console.log = (...args: unknown[]) => { lines.push(format(...args)); };
  try {
    await tab.goto(urlOf(page));
    const drive = driven(tab);
    // The prototype has CC once its script ran, the app once the shim has read the fixture; a preview page has none.
    if (!preview) await drive.waitForFunction('typeof CC !== "undefined"');
    await tab.clock.runFor(800);
    delete fromPrototype.cache[fromPrototype.resolve(file)];
    const steps = fromPrototype(file);
    await Promise.race([steps(drive, shot), new Promise((_, no) => { timer = setTimeout(() => no(new Error('timed out')), SIDE_TIMEOUT); })]);
  } catch (e) {
    // The first line says what failed, the call log's last wait says on what.
    const text = String((e as Error)?.message || e).replace(/\x1b\[[0-9;]*m/g, '').split('\n');
    const on = text.filter((l) => /^\s*- waiting for/.test(l)).pop()?.trim();
    errors.push(`step file stopped: ${text[0]}${on ? ` (${on})` : ''}`.slice(0, 400));
  } finally {
    clearTimeout(timer);
    console.log = log;
    await context.close();
  }
  return { lines, shots, errors, ms: Date.now() - started };
}

function compare(steps: string, a: Side, b: Side, dir: string): Pick<Result, 'problems' | 'allowed'> {
  const allow = allowFor(steps);
  const problems: string[] = [], allowed: string[] = [];
  const rules = (allow.lines || []).map((r) => ({ re: new RegExp(r.pattern, 'g'), why: r.why }));
  const norm = (line: string) => rules.reduce((l, r) => l.replace(r.re, '…'), line);

  const sameErrors = a.errors.join('\n') === b.errors.join('\n');
  if (allow.stale && sameErrors && a.errors.length) allowed.push(`both sides stop the same way, ${allow.stale}`);
  else {
    for (const e of a.errors) problems.push(`a: ${e}`);
    for (const e of b.errors) problems.push(`b: ${e}`);
  }

  const n = Math.max(a.lines.length, b.lines.length);
  for (let i = 0; i < n; i++) {
    const x = a.lines[i], y = b.lines[i];
    if (x === y) continue;
    if (x !== undefined && y !== undefined && norm(x) === norm(y)) {
      allowed.push(`line ${i + 1}: ${rules.find((r) => { r.re.lastIndex = 0; return r.re.test(x); })?.why}`);
      continue;
    }
    problems.push(`line ${i + 1}:\n    a: ${x ?? '(missing)'}\n    b: ${y ?? '(missing)'}`);
  }

  for (const name of new Set([...a.shots, ...b.shots])) {
    const pa = join(dir, 'a', name + '.png'), pb = join(dir, 'b', name + '.png');
    if (!existsSync(pa) || !existsSync(pb)) { problems.push(`shot ${name}: only on ${existsSync(pa) ? 'a' : 'b'}`); continue; }
    const d = diff(decode(readFileSync(pa)), decode(readFileSync(pb)));
    if (!d.pixels) continue;
    const ratio = d.pixels / d.total;
    const rule = allow.shots?.[name] || allow.shots?.['*'];
    writeFileSync(join(dir, 'diff-' + name + '.png'), encode(d.image));
    const what = `shot ${name}: ${d.pixels} pixels (${(ratio * 100).toFixed(3)} %) differ`;
    if (rule && d.pixels <= rule.pixels) allowed.push(`${what}, ${rule.why}`);
    else problems.push(what);
  }
  return { problems, allowed };
}

async function worker(file: string) {
  const { chromium } = fromPrototype('playwright') as typeof import('playwright');
  const steps = basename(file, '.cjs');
  const dir = join(opt.out, steps);
  // Raster work split over threads and tiles lands text on slightly different subpixels from run to run.
  const browser = await chromium.launch({
    args: ['--disable-lcd-text', '--disable-gpu-rasterization', '--disable-partial-raster', '--disable-threaded-animation', '--disable-checker-imaging', '--num-raster-threads=1'],
  });
  try {
    const own = allowFor(steps).page;
    const a = await runSide(browser, file, own ? join(prototype, 'dist', own) : opt.a, join(dir, 'a'), !!own);
    const b = await runSide(browser, file, own ? join(prototype, 'dist', own) : opt.b, join(dir, 'b'), !!own);
    const result: Result = { steps, a, b, ...compare(steps, a, b, dir) };
    const golden = join(here, 'golden', steps + '.txt');
    const text = a.lines.join('\n') + '\n';
    if (opt.update) {
      mkdirSync(join(here, 'golden'), { recursive: true });
      writeFileSync(golden, text);
    } else if (!existsSync(golden)) result.problems.push('no golden output yet, run with --update');
    else if (readFileSync(golden, 'utf8') !== text) result.problems.push('the prototype prints other lines than its golden output, check them and run with --update');
    writeFileSync(join(dir, 'result.json'), JSON.stringify(result, null, 1));
    process.send!(result);
  } finally {
    await browser.close();
  }
  process.exit(0);
}

async function main() {
  if (opt.a.endsWith('cannoncommander.html') && !existsSync(opt.a)) execFileSync(process.execPath, [join(prototype, 'build.mjs')], { stdio: 'inherit' });
  const wanted = opt.steps.split(',').filter(Boolean).map((s) => s.replace(/^steps-/, '').replace(/\.cjs$/, ''));
  const files = readdirSync(tools).filter((f) => /^steps-.+\.cjs$/.test(f))
    .filter((f) => !wanted.length || wanted.includes(f.slice(6, -4))).sort().map((f) => join(tools, f));
  if (!files.length) throw new Error('no step files match ' + opt.steps);
  mkdirSync(opt.out, { recursive: true });
  console.log(`parity: ${files.length} step files, a ${opt.a}, b ${opt.b}${opt.a === opt.b ? ' (the prototype against itself)' : ''}`);

  const results: Result[] = [];
  const queue = [...files];
  const one = async (): Promise<void> => {
    const file = queue.shift();
    if (!file) return;
    const result = await new Promise<Result>((done) => {
      const child = fork(import.meta.filename, ['--worker', file, '--a', opt.a, '--b', opt.b, '--out', opt.out, ...(opt.update ? ['--update'] : [])], { stdio: ['ignore', 'inherit', 'inherit', 'ipc'] });
      let got: Result | undefined;
      child.on('message', (m) => { got = m as Result; });
      child.on('exit', (code) => done(got || { steps: basename(file, '.cjs'), a: empty(), b: empty(), problems: [`worker ended with ${code}`], allowed: [] }));
    });
    results.push(result);
    const mark = result.problems.length ? 'DIFF' : 'ok  ';
    console.log(`${mark} ${result.steps.padEnd(22)} ${String(result.a.lines.length).padStart(4)} lines ${String(result.a.shots.length).padStart(3)} shots ${Math.round((result.a.ms + result.b.ms) / 1000)} s`
      + (result.allowed.length ? `, ${result.allowed.length} allowed` : '') + (result.problems.length ? `, ${result.problems.length} problems` : ''));
    for (const p of result.problems) console.log('     ' + p.replaceAll('\n', '\n     '));
    return one();
  };
  await Promise.all(Array.from({ length: Math.max(1, Number(opt.jobs)) }, one));

  results.sort((x, y) => x.steps.localeCompare(y.steps));
  writeFileSync(join(opt.out, 'report.json'), JSON.stringify(results.map(({ steps, problems, allowed, a, b }) => ({ steps, problems, allowed, lines: a.lines.length, shots: a.shots.length, ms: a.ms + b.ms })), null, 1));
  const bad = results.filter((r) => r.problems.length);
  const sum = (f: (r: Result) => number) => results.reduce((n, r) => n + f(r), 0);
  console.log(`\n${results.length - bad.length} of ${results.length} step files match, ${sum((r) => r.a.lines.length)} lines and ${sum((r) => r.a.shots.length)} screenshots compared, `
    + `${sum((r) => r.allowed.length)} allowed differences. Report: ${join(opt.out, 'report.json')}`);
  process.exitCode = bad.length ? 1 : 0;
}

const empty = (): Side => ({ lines: [], shots: [], errors: [], ms: 0 });

if (opt.worker) await worker(opt.worker);
else await main();
