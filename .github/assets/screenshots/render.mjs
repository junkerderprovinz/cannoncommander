// Builds the README pictures in the style of the house's store images: the web interface in dark mode, in a tilted
// browser window or desktop window on the dark relief ground with the logo, beside a heading and a grey line. Each
// picture is 1920x1000, rendered at twice the size so the text stays sharp through the tilt.
//
// `capture` first takes the interface from the fixture server, the real app against the prototype's made-up tree,
// at 1440x700 in twice the pixels, into captures/. The address bar and the MCP endpoint both read nas.local:8480,
// the template's default port. Without an argument the pictures are drawn from those captures.
//
// Run: node .github/assets/screenshots/render.mjs [capture]
import { spawn } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const here = import.meta.dirname;
const repo = resolve(here, '..', '..', '..');
const captures = join(here, 'captures');
const { chromium } = createRequire(join(repo, 'prototype', 'package.json'))('playwright');

const HOST = 'nas.local';
const PORT = 8480;
const VIEW = { width: 1440, height: 700 };
const SERVER = 'CannonCommander on Unraid, in your browser';
const DESKTOP = 'CannonCommander for Windows, macOS and Linux';

const sleep = (ms) => new Promise((done) => setTimeout(done, ms));
const panel = (tab, i) => tab.locator(`[data-panel="${i}"]`);

// A click low in a panel's list makes it the active one without touching a row.
async function focus(tab, i) {
  const box = await panel(tab, i).boundingBox();
  await tab.mouse.click(box.x + box.width / 2, box.y + box.height - 70);
}

async function go(tab, path) {
  await tab.keyboard.press('Control+l');
  await sleep(300);
  await tab.keyboard.type(path);
  await tab.keyboard.press('Enter');
  await sleep(800);
}

const row = (tab, name) => tab.getByText(name, { exact: true }).first();

async function settings(tab, page) {
  await tab.getByRole('button', { name: 'Settings' }).click();
  await sleep(800);
  await tab.getByText(page, { exact: true }).first().click();
  await sleep(1000);
}

// In the order of the download buttons: the server first, then the desktop app.
const SHOTS = [
  {
    name: 'panels', frame: 'browser', heading: "Your server's files, <em>side by side</em>",
    alt: 'Two panels in a browser: the downloads share with two more tabs on the left, the backups share on the right, and shortcuts, remotes and the folder tree in the sidebar',
    caption: 'Tabs in both panels, and badges for the files that still sit on the cache.',
    async prepare(tab) {
      await focus(tab, 1);
      await go(tab, '/mnt/user/backups');
      await focus(tab, 0);
      await tab.keyboard.press('Control+t');
      await go(tab, '/mnt/user/fotos');
      await tab.keyboard.press('Control+t');
      await go(tab, '/mnt/user/isos');
      await tab.keyboard.press('Control+Tab');
      await sleep(500);
      await row(tab, 'ubuntu-24.04.3-desktop-amd64.iso').click();
    },
  },
  {
    name: 'copy', frame: 'browser', heading: 'Press F5. <em>Keep working.</em>',
    alt: 'The copy window for a film and a zip file going from downloads to backups, with the choice between a copy and a hard link and the rule for files that already exist',
    caption: 'Copies run as jobs in the background, and a hard link saves the space where Unraid allows one.',
    async prepare(tab) {
      await focus(tab, 1);
      await go(tab, '/mnt/user/backups');
      await focus(tab, 0);
      await row(tab, 'Big.Buck.Bunny.2008.1080p.mkv').click();
      await row(tab, 'Fotos-Urlaub-2026.zip').click({ modifiers: ['Control'] });
      await row(tab, 'Big.Buck.Bunny.2008.1080p.mkv').click({ modifiers: ['Control'] });
      await tab.keyboard.press('F5');
    },
  },
  {
    name: 'remotes', frame: 'browser', heading: 'Any cloud, <em>just another folder</em>',
    alt: 'The window for adding a remote, with a search field and a grid of cloud providers, each with its logo',
    caption: 'Every provider rclone speaks, browsed like a share on the server.',
    async prepare(tab) {
      await settings(tab, 'Remotes');
      await tab.getByText('Add a remote', { exact: true }).first().click();
    },
  },
  {
    name: 'preview', frame: 'browser', heading: 'See the photo, <em>not just its name</em>',
    alt: 'Two panels of holiday photos with the preview pane on the right, showing the picture with its size, format, camera, date and place',
    caption: 'The preview pane shows the file under the cursor, with the camera data of a photo.',
    prefs: { sidePane: true },
    async prepare(tab) {
      await tab.keyboard.press('Control+b');
      await sleep(600);
      await focus(tab, 1);
      await go(tab, '/mnt/user/fotos');
      await focus(tab, 0);
      await go(tab, '/mnt/user/fotos/Urlaub');
      await row(tab, 'DSC_0004.jpg').click();
    },
  },
  {
    name: 'gallery', frame: 'browser', heading: 'Nine views. <em>Every folder keeps its own.</em>',
    alt: 'A folder of photos from 2026 shown as large tiles in one wide panel',
    caption: 'Details, columns, tiles, a gallery and more, remembered per folder.',
    prefs: { folderViews: { '/mnt/user/fotos/2026': 'large' } },
    async prepare(tab) {
      await focus(tab, 0);
      await go(tab, '/mnt/user/fotos/2026');
      await tab.keyboard.press('Control+1');
      await sleep(800);
      await row(tab, 'IMG_0203.jpg').click();
    },
  },
  {
    name: 'editor', frame: 'browser', heading: 'Fix the compose file. <em>No SSH needed.</em>',
    alt: 'The editor with a compose file and a Go file open in tabs, syntax colours, line numbers and a status bar',
    caption: 'F4 opens the editor, with tabs, syntax colours and find and replace.',
    async prepare(tab) {
      await focus(tab, 1);
      await go(tab, '/mnt/user/dokumente');
      await row(tab, 'docker-compose.yml').click();
      await row(tab, 'server.go').click({ modifiers: ['Control'] });
      await row(tab, 'docker-compose.yml').click({ modifiers: ['Control'] });
      await tab.keyboard.press('F4');
    },
  },
  {
    name: 'mcp', frame: 'browser', heading: 'Let your AI in, <em>with its own key</em>',
    alt: 'The MCP server card in the settings: its status, the endpoint, a button to connect a client and two keys with their calls today and last use',
    caption: 'The built-in MCP server lets Claude, ChatGPT, Cursor and others work with your files, each through its own key.',
    async prepare(tab) {
      await tab.evaluate(() => fetch('/api/v1/mcp/keys/a1b2c3d4e5f60718293a4b5c6d7e8f90', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ label: 'Claude Code on the laptop' }) }));
      await settings(tab, 'Integrations');
      await tab.waitForFunction(() => /2 keys active/.test(document.body.innerText));
      await tab.evaluate(() => {
        const badge = [...document.querySelectorAll('.glim-badge')].find((b) => /^MCP server/i.test(b.textContent.trim()));
        const card = badge.closest('.glim-notch-card');
        card.style.scrollMarginTop = '70px';
        card.scrollIntoView();
      });
    },
  },
  {
    name: 'desktop', frame: 'window', heading: 'Explorer or F&#8209;keys? <em>Your call.</em>',
    alt: 'The desktop app at its first start, asking whether it should work as in Explorer or like a commander',
    caption: 'The desktop app works on your own computer and asks once how you like to work.',
    desktop: true,
  },
];

// The fixture keeps /api/v1 in memory, so a server of its own starts every run from the same tree.
async function fixture() {
  const vite = join(repo, 'frontend', 'node_modules', 'vite', 'bin', 'vite.js');
  const child = spawn(process.execPath, [vite, '--mode', 'fixture', '--host', '127.0.0.1', '--port', String(PORT), '--strictPort'],
    { cwd: join(repo, 'frontend'), env: { ...process.env, __VITE_ADDITIONAL_SERVER_ALLOWED_HOSTS: HOST }, stdio: 'ignore' });
  process.once('exit', () => child.kill());
  for (let waited = 0; !(await fetch(`http://127.0.0.1:${PORT}/api/v1/me`).then((r) => r.ok, () => false)); waited += 250) {
    if (child.exitCode !== null || waited > 120_000) throw new Error(`the fixture server on port ${PORT} did not come up`);
    await sleep(250);
  }
  return child;
}

// The fixture's sample files were written in German, and the compose file names another port than the template.
const ENGLISH = [
  ['# Nur im internen Netz, CannonCommander spricht ihn über den Namen an', '# Internal network only, CannonCommander reaches it by name'],
  ['8443:8080', '8480:80'],
];

// What only the desktop app answers, as an empty computer: the fixture server stands for a server.
const ON_THE_COMPUTER = {
  '/api/v1/filemanager': { on: false },
  '/api/v1/fs/templates': { templates: [] },
  '/api/v1/places': { folders: [], volumes: [] },
  '/api/v1/shell': { menu: '', properties: false, runAs: [] },
  '/api/v1/clipboard': { files: [], cut: false, skipped: 0 },
};

async function capture(browser, shot) {
  const ctx = await browser.newContext({ viewport: VIEW, deviceScaleFactor: 2, colorScheme: 'dark', locale: 'en-US', timezoneId: 'Europe/Vienna' });
  const tab = await ctx.newPage();
  const now = JSON.parse(readFileSync(join(repo, 'testing', 'fixture', 'prototype.json'), 'utf8')).now;
  await tab.clock.install({ time: now + 60_000 });
  await tab.addInitScript((prefs) => {
    if (sessionStorage.getItem('seeded')) return;
    localStorage.setItem('cannoncommander-prefs', JSON.stringify(prefs));
    sessionStorage.setItem('seeded', '1');
  }, { language: 'en', theme: 'dark', motion: 'subtle', ...shot.prefs });
  await tab.route('**/api/v1/**', async (route) => {
    if (route.request().resourceType() === 'eventsource') return route.continue();
    const path = new URL(route.request().url()).pathname;
    if (shot.desktop && path in ON_THE_COMPUTER) return route.fulfill({ json: ON_THE_COMPUTER[path] });
    // The fetch runs in Node, which knows nothing of the browser's mapping for the made-up host.
    const res = await route.fetch({ url: route.request().url().replace(`//${HOST}:`, '//127.0.0.1:') });
    const type = res.headers()['content-type'] || '';
    if (!/json|text/.test(type)) return route.fulfill({ response: res });
    let body = await res.text();
    if (shot.desktop && path === '/api/v1/system') {
      body = JSON.stringify({ ...JSON.parse(body), deployment: 'desktop', os: 'linux' });
    }
    for (const [from, to] of ENGLISH) body = body.replaceAll(from, to);
    await route.fulfill({ response: res, body });
  });
  await tab.goto(`http://${HOST}:${PORT}/`, { timeout: 120_000 });
  await tab.waitForLoadState('networkidle');
  await sleep(1500);
  await shot.prepare?.(tab);
  // The pointer of the last click would light up whatever sits under it, and the windows open with a short motion.
  await tab.mouse.move(VIEW.width - 1, VIEW.height - 1);
  await sleep(1500);
  await tab.screenshot({ path: join(captures, `${shot.name}.png`), caret: 'hide' });
  await ctx.close();
}

async function cached(file, url) {
  const path = join(tmpdir(), `CannonCommander-${file}`);
  if (!existsSync(path)) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${file}: fetch ${res.status}`);
    writeFileSync(path, Buffer.from(await res.arrayBuffer()));
  }
  return readFileSync(path);
}

const dataUrl = (buf, type) => `data:${type};base64,${buf.toString('base64')}`;

async function style() {
  const fonts = 'https://github.com/google/fonts/raw/main/ofl';
  const bree = dataUrl(await cached('BreeSerif-Regular.ttf', `${fonts}/breeserif/BreeSerif-Regular.ttf`), 'font/ttf');
  const lato = dataUrl(await cached('Lato-Regular.ttf', `${fonts}/lato/Lato-Regular.ttf`), 'font/ttf');
  return `
@font-face { font-family: "Bree Serif"; src: url(${bree}); }
@font-face { font-family: Lato; src: url(${lato}); }
* { box-sizing: border-box; margin: 0; }
body { position: relative; width: 1920px; height: 1000px; overflow: hidden; font-family: Lato, sans-serif; background: #0c0c0b; }
.backdrop, .backdrop * { position: absolute; }
.backdrop { inset: 0; overflow: hidden; }
.wall { inset: 0; background: radial-gradient(55% 60% at 62% 35%, #26231d, #0f0e0c 72%); }
.mark { width: 50%; left: -9%; top: -6%; transform: rotate(-10deg); opacity: .32;
  filter: grayscale(1) brightness(.36) contrast(1.2) drop-shadow(-2px -2px 0 rgba(255,255,255,.16)) drop-shadow(12px 18px 26px rgba(0,0,0,.85)); }
.vignette { inset: 0; box-shadow: inset 0 0 200px rgba(0,0,0,.6); }
.copy { position: absolute; left: 84px; top: 0; bottom: 0; width: 470px; display: flex; flex-direction: column; justify-content: center; gap: 28px; }
.copy img { width: 92px; }
h1 { font: 400 64px/1.12 "Bree Serif", serif; color: #f4f4f4; text-wrap: balance; }
h1 em { font-style: normal; color: #FCC419; }
.sub { font-size: 28px; line-height: 1.35; color: #9d9481; }
.stage { position: absolute; left: 600px; width: 1270px; perspective: 2400px; }
.floor { position: absolute; left: 12%; right: 12%; bottom: -4%; height: 8%; border-radius: 50%; background: rgba(0,0,0,.8); filter: blur(30px); }
.win { position: absolute; inset: 0; transform: rotateY(-8deg) rotateX(2deg); border-radius: 14px; overflow: hidden; background: #161616;
  box-shadow: 0 0 0 1px #3c3c3c, 0 2px 4px rgba(0,0,0,.35), 0 18px 36px rgba(0,0,0,.45), 0 52px 100px rgba(0,0,0,.55); }
.win > img { display: block; width: 1270px; height: 617px; }
.bar { position: relative; display: flex; align-items: center; background: #242424; color: #d6d6d6; font: 400 17px/1 Lato, sans-serif; }
.bar img { width: 22px; height: 22px; }
.app { height: 44px; gap: 10px; padding-left: 16px; border-bottom: 1px solid #111; }
.ctl { position: absolute; top: 0; width: 46px; height: 44px; }
.ctl::before { content: ""; position: absolute; left: 17px; top: 21px; width: 12px; height: 1.5px; background: #bdbdbd; }
.ctl.min { right: 92px; }
.ctl.max { right: 46px; }
.ctl.max::before { top: 16px; height: 12px; background: none; border: 1.5px solid #bdbdbd; }
.ctl.close { right: 0; }
.ctl.close::before, .ctl.close::after { content: ""; position: absolute; left: 16px; top: 21px; width: 14px; height: 1.5px; background: #bdbdbd; }
.ctl.close::before { transform: rotate(45deg); }
.ctl.close::after { transform: rotate(-45deg); }
.tabs { height: 42px; padding: 7px 0 0 12px; background: #1c1c1c; align-items: flex-end; }
.tab { display: flex; align-items: center; gap: 10px; height: 35px; padding: 0 70px 0 14px; border-radius: 10px 10px 0 0; background: #2c2c2c; font-size: 15px; }
.tab img { width: 18px; height: 18px; }
.address { height: 46px; gap: 14px; padding: 0 14px; background: #2c2c2c; border-bottom: 1px solid #111; }
.nav { width: 10px; height: 10px; border-left: 2px solid #9a9a9a; border-bottom: 2px solid #9a9a9a; transform: rotate(45deg); margin: 0 4px; }
.nav.fwd { transform: rotate(-135deg); border-color: #5a5a5a; }
.url { flex: 1; height: 32px; border-radius: 16px; background: #1a1a1a; padding-left: 18px; display: flex; align-items: center; font-size: 15px; color: #c9c9c9; }
.glare { position: absolute; inset: 0; pointer-events: none;
  background: linear-gradient(118deg, rgba(255,255,255,.09) 0%, rgba(255,255,255,.03) 28%, rgba(255,255,255,0) 42%); }
`;
}

function picture(css, logo, shot) {
  const browser = shot.frame === 'browser';
  const chrome = browser
    ? `<div class="bar tabs"><div class="tab"><img src="${logo}"><span>CannonCommander</span></div></div>
       <div class="bar address"><i class="nav"></i><i class="nav fwd"></i><div class="url">${HOST}:${PORT}</div></div>`
    : `<div class="bar app"><img src="${logo}"><span>CannonCommander</span><i class="ctl min"></i><i class="ctl max"></i><i class="ctl close"></i></div>`;
  const bar = browser ? 89 : 45;
  const shown = dataUrl(readFileSync(join(captures, `${shot.name}.png`)), 'image/png');
  return `<!doctype html><html><head><meta charset="utf-8"><style>${css}</style></head><body>
<div class="backdrop"><div class="wall"></div><img class="mark" src="${logo}"><div class="vignette"></div></div>
<div class="copy"><img src="${logo}"><h1>${shot.heading}</h1><p class="sub">${shot.desktop ? DESKTOP : SERVER}</p></div>
<div class="stage" style="top:${browser ? 148 : 170}px;height:${617 + bar}px"><div class="floor"></div>
  <div class="win">${chrome}<img src="${shown}"><div class="glare"></div></div>
</div>
</body></html>`;
}

// Drawn at twice the size and scaled down in a second page.
async function render(browser) {
  const css = await style();
  const logo = dataUrl(readFileSync(join(repo, '.github', 'assets', 'logo.svg')), 'image/svg+xml');
  for (const shot of SHOTS) {
    const big = await browser.newPage({ viewport: { width: 1920, height: 1000 }, deviceScaleFactor: 2 });
    await big.setContent(picture(css, logo, shot));
    await big.evaluate(() => document.fonts.ready);
    await big.evaluate(() => Promise.all([...document.images].map((img) => img.decode())));
    const png = await big.screenshot();
    await big.close();
    const small = await browser.newPage({ viewport: { width: 1920, height: 1000 } });
    await small.setContent(`<body style="margin:0"><img src="${dataUrl(png, 'image/png')}" style="display:block;width:1920px;height:1000px">`);
    await small.locator('img').evaluate((img) => img.decode());
    await small.screenshot({ path: join(here, `${shot.name}.png`) });
    await small.close();
    console.log(`wrote ${shot.name}.png`);
  }
}

if (process.argv[2] === 'capture') {
  mkdirSync(captures, { recursive: true });
  const server = await fixture();
  const browser = await chromium.launch({ args: ['--disable-lcd-text', `--host-resolver-rules=MAP ${HOST} 127.0.0.1`] });
  try {
    for (const shot of SHOTS) {
      await capture(browser, shot);
      console.log(`captured ${shot.name}.png`);
    }
  } finally {
    await browser.close();
    server.kill();
  }
} else {
  const browser = await chromium.launch();
  try {
    await render(browser);
  } finally {
    await browser.close();
  }
}
