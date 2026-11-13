// Takes the pictures of the documentation site from the real interface, running against the fixture server and its
// made-up tree, the way .github/assets/screenshots/render.mjs takes the README pictures. Each one is the bare
// interface at 1440x900, without a frame, saved to docs/assets/screenshots/. A shot marked light is taken in both
// themes, as name-dark.png and name-light.png.
//
// Needs npm ci in frontend/ and prototype/. A fixture server already listening on the port is used as it is.
//
// Run: node scripts/docs_screenshots.mjs [name ...]
import { spawn } from 'node:child_process';
import { mkdirSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';

const repo = resolve(import.meta.dirname, '..');
const out = join(repo, 'docs', 'assets', 'screenshots');
const { chromium } = createRequire(join(repo, 'prototype', 'package.json'))('playwright');

const HOST = 'nas.local';
const PORT = 8480;
const VIEW = { width: 1440, height: 900 };

const sleep = (ms) => new Promise((done) => setTimeout(done, ms));
const panel = (tab, i) => tab.locator(`[data-panel="${i}"]`);
const row = (tab, i, name) => panel(tab, i).getByText(name, { exact: true }).first();
// A plain click only moves the cursor, so the first name is marked with Ctrl as well.
async function mark(tab, i, first, ...rest) {
  await row(tab, i, first).click();
  for (const name of [...rest, first]) await row(tab, i, name).click({ modifiers: ['Control'] });
}
const dialog = (tab) => tab.locator('.glim-window').last();

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

async function pair(tab, left, right) {
  await focus(tab, 1);
  await go(tab, right);
  await focus(tab, 0);
  await go(tab, left);
}

async function menu(tab, top, item) {
  await tab.getByRole('menuitem', { name: top, exact: true }).click();
  await sleep(400);
  if (item) {
    await tab.getByRole('menuitem', { name: item, exact: true }).first().click();
    await sleep(900);
  }
}

async function settings(tab, page) {
  await tab.getByRole('button', { name: 'Settings' }).click();
  await sleep(800);
  await tab.getByText(page, { exact: true }).first().click();
  await sleep(1000);
}

// Brings the settings card with this badge to the top of the page.
async function card(tab, badge) {
  await tab.evaluate((name) => {
    const b = [...document.querySelectorAll('.glim-badge')].find((x) => x.textContent.trim().toLowerCase() === name.toLowerCase());
    const c = b.closest('.glim-notch-card');
    c.style.scrollMarginTop = '70px';
    c.scrollIntoView();
  }, badge);
  await sleep(500);
}

async function toggle(tab, badge) {
  const c = tab.locator('.glim-notch-card').filter({ has: tab.locator('.glim-badge', { hasText: new RegExp(`^${badge}$`, 'i') }) });
  await c.locator('[role="switch"], input[type="checkbox"]').first().click({ force: true });
  await sleep(1200);
}

const button = (tab, name) => tab.getByRole('button', { name, exact: true }).last();

// An account with a password, for the pages that only show up once one is set.
const WITH_PASSWORD = { '/api/v1/me': (me) => ({ ...me, hasPassword: true, signIn: true }) };
const SIGNED_OUT = { '/api/v1/me': () => ({ status: 401 }) };

// What only the desktop app answers, as an empty computer: the fixture server stands for a server.
const ON_THE_COMPUTER = {
  '/api/v1/filemanager': { on: false },
  '/api/v1/fs/templates': { templates: [] },
  '/api/v1/places': { folders: [], volumes: [] },
  '/api/v1/shell': { menu: '', properties: false, runAs: [] },
  '/api/v1/clipboard': { files: [], cut: false, skipped: 0 },
};

const SHOTS = [
  {
    name: 'panels', light: true,
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
      await row(tab, 0, 'ubuntu-24.04.3-desktop-amd64.iso').click();
    },
  },
  { name: 'menu-view', prepare: (tab) => menu(tab, 'View') },
  { name: 'menu-tools', prepare: (tab) => menu(tab, 'Tools') },
  {
    name: 'palette',
    async prepare(tab) {
      await tab.keyboard.press('Control+k');
      await sleep(800);
      await tab.keyboard.type('compare');
    },
  },
  {
    name: 'shortcuts',
    async prepare(tab) {
      await focus(tab, 0);
      await tab.keyboard.press('F1');
      await sleep(1000);
    },
  },
  { name: 'settings-keys', prepare: (tab) => settings(tab, 'Keyboard shortcuts') },
  {
    name: 'copy',
    async prepare(tab) {
      await pair(tab, '/mnt/user/downloads', '/mnt/user/backups');
      await mark(tab, 0, 'Big.Buck.Bunny.2008.1080p.mkv', 'Fotos-Urlaub-2026.zip');
      await tab.keyboard.press('F5');
    },
  },
  {
    name: 'quicklook', prefs: { quickInfo: true },
    async prepare(tab) {
      await pair(tab, '/mnt/user/fotos/Urlaub', '/mnt/user/fotos/2026');
      await row(tab, 0, 'DSC_0006.jpg').click();
      await tab.keyboard.press('Space');
      await sleep(3000);
    },
  },
  {
    name: 'preview-pane', prefs: { sidePane: true },
    async prepare(tab) {
      await tab.keyboard.press('Control+b');
      await sleep(600);
      // Without the sidebar the panels move, so only the right one is steered.
      await focus(tab, 1);
      await go(tab, '/mnt/user/fotos/Urlaub');
      await row(tab, 1, 'DSC_0004.jpg').click();
    },
  },
  {
    name: 'menu-layout',
    async prepare(tab) {
      await focus(tab, 0);
      await go(tab, '/mnt/user/fotos/2026');
      await menu(tab, 'View');
      await tab.getByRole('menuitem', { name: 'Layout' }).hover();
    },
  },
  {
    name: 'gallery', light: true, prefs: { folderViews: { '/mnt/user/fotos/2026': 'large' } },
    async prepare(tab) {
      await focus(tab, 0);
      await go(tab, '/mnt/user/fotos/2026');
      await tab.keyboard.press('Control+1');
      await sleep(800);
      await row(tab, 0, 'IMG_0203.jpg').click();
    },
  },
  {
    name: 'columns',
    async prepare(tab) {
      await focus(tab, 0);
      await menu(tab, 'View', 'Choose columns');
    },
  },
  {
    name: 'quick-filter',
    async prepare(tab) {
      await pair(tab, '/mnt/user/fotos/2026', '/mnt/user/fotos');
      await tab.keyboard.press('/');
      await sleep(300);
      await tab.keyboard.type('IMG_021');
    },
  },
  {
    name: 'search',
    async prepare(tab) {
      await focus(tab, 0);
      await tab.keyboard.press('Alt+F7');
      await sleep(1000);
      await dialog(tab).locator('input').first().fill('*.iso');
      await sleep(300);
    },
  },
  {
    name: 'compare',
    async prepare(tab) {
      await pair(tab, '/mnt/user/dokumente', '/mnt/user/backups/dokumente');
      await tab.keyboard.press('Control+Shift+c');
      await sleep(1500);
    },
  },
  {
    name: 'compare-marked', toasts: true,
    async prepare(tab) {
      await pair(tab, '/mnt/user/dokumente', '/mnt/user/backups/dokumente');
      await tab.keyboard.press('Control+Shift+c');
      await sleep(1500);
      await button(tab, 'Mark differences').click();
      await sleep(4500);
    },
  },
  {
    name: 'sync',
    async prepare(tab) {
      await pair(tab, '/mnt/user/dokumente', '/mnt/user/backups/dokumente');
      await menu(tab, 'Tools', 'Synchronise');
      await sleep(800);
    },
  },
  {
    name: 'duplicates',
    async prepare(tab) {
      await pair(tab, '/mnt/user/fotos', '/mnt/user/backups');
      await menu(tab, 'Tools', 'Duplicate files');
      await sleep(1500);
    },
  },
  {
    name: 'disk-usage',
    async prepare(tab) {
      await pair(tab, '/mnt/user', '/mnt/user/backups');
      await tab.keyboard.press('Alt+d');
      await sleep(2000);
    },
  },
  {
    name: 'rename',
    async prepare(tab) {
      await pair(tab, '/mnt/user/fotos/Urlaub', '/mnt/user/fotos');
      await tab.keyboard.press('Control+a');
      await tab.keyboard.press('Control+m');
      await sleep(1200);
      const name = dialog(tab).locator('input').first();
      await name.fill('Urlaub-[C3]');
      await sleep(800);
    },
  },
  {
    name: 'pack',
    async prepare(tab) {
      await pair(tab, '/mnt/user/dokumente', '/mnt/user/backups');
      await mark(tab, 0, 'docker-compose.yml', 'server.go');
      await tab.keyboard.press('Alt+p');
      await sleep(1200);
    },
  },
  {
    name: 'archive',
    async prepare(tab) {
      await pair(tab, '/mnt/user/downloads', '/mnt/user/backups');
      await row(tab, 0, 'Fotos-Urlaub-2026.zip').dblclick();
      await sleep(1500);
    },
  },
  {
    name: 'checksums',
    async prepare(tab) {
      await pair(tab, '/mnt/user/isos', '/mnt/user/backups');
      await tab.keyboard.press('Control+a');
      await menu(tab, 'File', 'Make checksums');
      await sleep(2500);
    },
  },
  {
    name: 'editor',
    async prepare(tab) {
      await focus(tab, 1);
      await go(tab, '/mnt/user/dokumente');
      await mark(tab, 1, 'docker-compose.yml', 'server.go');
      await tab.keyboard.press('F4');
      await sleep(2500);
    },
  },
  {
    name: 'viewer',
    async prepare(tab) {
      await pair(tab, '/mnt/user/fotos/Urlaub', '/mnt/user/fotos');
      await row(tab, 0, 'DSC_0004.jpg').click();
      await tab.keyboard.press('F3');
      await sleep(2500);
    },
  },
  {
    name: 'trash',
    async prepare(tab) {
      await menu(tab, 'Go', 'Trash');
      await sleep(800);
    },
  },
  { name: 'settings-general', prepare: (tab) => settings(tab, 'General') },
  {
    name: 'share',
    async prepare(tab) {
      await pair(tab, '/mnt/user/fotos/Urlaub', '/mnt/user/backups');
      await row(tab, 0, 'DSC_0004.jpg').click();
      await menu(tab, 'File', 'Share');
      await dialog(tab).locator('textarea').fill('Here are the photos from the weekend');
    },
  },
  { name: 'settings-shares', prepare: (tab) => settings(tab, 'Shares') },
  {
    name: 'context-menu',
    async prepare(tab) {
      await pair(tab, '/mnt/user/downloads', '/mnt/user/backups');
      await row(tab, 0, 'setup-notes.md').click({ button: 'right' });
      await sleep(600);
      await tab.getByRole('menuitem', { name: 'Colour' }).hover();
      await sleep(800);
    },
    keepPointer: true,
  },
  {
    name: 'tags',
    async prepare(tab) {
      await pair(tab, '/mnt/user/downloads', '/mnt/user/backups');
      await row(tab, 0, 'setup-notes.md').click({ button: 'right' });
      await sleep(600);
      await tab.getByRole('menuitem', { name: 'Tags' }).click();
      await sleep(800);
    },
  },
  {
    name: 'properties',
    async prepare(tab) {
      await pair(tab, '/mnt/user/downloads', '/mnt/user/backups');
      await row(tab, 0, 'setup-notes.md').click();
      await tab.keyboard.press('Alt+Enter');
      await sleep(1200);
    },
  },
  { name: 'remotes', prepare: (tab) => settings(tab, 'Remotes') },
  {
    name: 'remote-add',
    async prepare(tab) {
      await settings(tab, 'Remotes');
      await tab.getByText('Add a remote', { exact: true }).first().click();
      await sleep(1500);
    },
  },
  {
    name: 'remote-sftp',
    async prepare(tab) {
      await settings(tab, 'Remotes');
      await tab.getByText('Add a remote', { exact: true }).first().click();
      await sleep(1500);
      await dialog(tab).getByText('SFTP', { exact: true }).first().click();
      await sleep(1000);
      const fields = dialog(tab).locator('input[type=text], input:not([type])');
      await fields.nth(0).fill('192.168.1.20');
      await fields.nth(1).fill('backup');
    },
  },
  {
    name: 'remote-browse',
    async prepare(tab) {
      await focus(tab, 0);
      await go(tab, '/remotes/gdrive');
      await focus(tab, 1);
      await go(tab, '/mnt/user/dokumente');
      await focus(tab, 0);
    },
  },
  { name: 'account-no-password', prepare: (tab) => settings(tab, 'Account & security') },
  {
    name: 'set-password',
    async prepare(tab) {
      await settings(tab, 'Account & security');
      await tab.getByText('Set a password').click();
      await sleep(800);
    },
  },
  {
    name: 'sign-in', api: { ...SIGNED_OUT, '/api/v1/auth/options': (o) => ({ ...o, server: 'Tower' }) },
    async prepare(tab) {
      await tab.locator('#cl-user').fill('anna');
    },
  },
  {
    name: 'sign-in-oidc', api: { ...SIGNED_OUT, '/api/v1/auth/options': (o) => ({ ...o, server: 'Tower', oidc: { label: 'OpenID Connect', url: '#' } }) },
  },
  { name: 'account', api: WITH_PASSWORD, host: 'localhost', prepare: (tab) => settings(tab, 'Account & security') },
  {
    name: 'account-tokens', api: WITH_PASSWORD, host: 'localhost',
    async prepare(tab) {
      await settings(tab, 'Account & security');
      await card(tab, 'API tokens');
    },
  },
  {
    name: 'token-add', api: WITH_PASSWORD,
    async prepare(tab) {
      await settings(tab, 'Account & security');
      await card(tab, 'API tokens');
      await button(tab, 'Create a token').click();
      await sleep(800);
    },
  },
  { name: 'users', prepare: (tab) => settings(tab, 'Users & groups') },
  {
    name: 'user-add',
    async prepare(tab) {
      await settings(tab, 'Users & groups');
      await tab.getByText('Add a user').click();
      await sleep(800);
      const fields = dialog(tab).locator('input');
      await fields.nth(0).fill('lena');
      await fields.nth(1).fill('Lena');
    },
  },
  { name: 'rules', prepare: (tab) => settings(tab, 'Access rules') },
  {
    name: 'rule-add',
    async prepare(tab) {
      await settings(tab, 'Access rules');
      await tab.getByText('Add a rule').click();
      await sleep(800);
    },
  },
  {
    name: 'office',
    async prepare(tab) {
      await settings(tab, 'Integrations');
      await card(tab, 'Office');
    },
  },
  {
    name: 'webdav',
    async prepare(tab) {
      await settings(tab, 'Integrations');
      await card(tab, 'WebDAV and rclone');
      await toggle(tab, 'WebDAV and rclone');
    },
  },
  {
    name: 'terminal-switch',
    async prepare(tab) {
      await settings(tab, 'Integrations');
      await card(tab, 'Terminal');
      await toggle(tab, 'Terminal');
    },
  },
  {
    name: 'mcp',
    async prepare(tab) {
      await settings(tab, 'Integrations');
      await tab.waitForFunction(() => /2 keys active/.test(document.body.innerText));
      await card(tab, 'MCP server');
    },
  },
  {
    name: 'mcp-clients',
    async prepare(tab) {
      await settings(tab, 'Integrations');
      await card(tab, 'MCP server');
      await tab.getByText('Choose a client').click();
      await sleep(1000);
    },
  },
  {
    name: 'mcp-connect',
    async prepare(tab) {
      await settings(tab, 'Integrations');
      await card(tab, 'MCP server');
      await tab.getByText('Choose a client').click();
      await sleep(1000);
      await dialog(tab).getByText('Claude Code', { exact: true }).click();
      await sleep(1200);
    },
  },
  { name: 'sources', prepare: (tab) => settings(tab, 'Sources') },
  { name: 'system', prepare: (tab) => settings(tab, 'System') },
  { name: 'info', prepare: (tab) => settings(tab, 'Info') },
  {
    name: 'activity',
    async prepare(tab) {
      await menu(tab, 'Go', 'Activity');
      await sleep(800);
    },
  },
  { name: 'desktop', desktop: true },
  // The fixture keeps what a job changed, so the copy that runs comes last.
  {
    name: 'jobs',
    async prepare(tab) {
      await pair(tab, '/mnt/user/downloads', '/mnt/user/backups');
      await row(tab, 0, 'debian-13.1.0-amd64-netinst.iso').click();
      await tab.keyboard.press('F5');
      await sleep(1000);
      await button(tab, 'Copy').click();
      await sleep(300);
      await menu(tab, 'Tools', 'Jobs');
    },
  },
];

// The fixture keeps /api/v1 in memory, so a server of its own starts from the same tree.
async function fixture() {
  const up = () => fetch(`http://127.0.0.1:${PORT}/api/v1/system`).then((r) => r.ok, () => false);
  if (await up()) return null;
  const vite = join(repo, 'frontend', 'node_modules', 'vite', 'bin', 'vite.js');
  const child = spawn(process.execPath, [vite, '--mode', 'fixture', '--host', '127.0.0.1', '--port', String(PORT), '--strictPort'],
    { cwd: join(repo, 'frontend'), env: { ...process.env, __VITE_ADDITIONAL_SERVER_ALLOWED_HOSTS: HOST }, stdio: 'ignore' });
  process.once('exit', () => child.kill());
  for (let waited = 0; !(await up()); waited += 250) {
    if (child.exitCode !== null || waited > 120_000) throw new Error(`the fixture server on port ${PORT} did not come up`);
    await sleep(250);
  }
  return child;
}

// The fixture's sample files and key names were written in German, and the compose file names another port than the
// template.
const ENGLISH = [
  ['# Nur im internen Netz, CannonCommander spricht ihn über den Namen an', '# Internal network only, CannonCommander reaches it by name'],
  ['8443:8080', '8480:80'],
  ['Claude Code auf dem Laptop', 'Claude Code on the laptop'],
  ['"name":"Handy"', '"name":"Phone"'],
];

async function capture(browser, shot, theme) {
  const ctx = await browser.newContext({ viewport: VIEW, colorScheme: theme, locale: 'en-US', timezoneId: 'Europe/Vienna' });
  const tab = await ctx.newPage();
  const now = JSON.parse(readFileSync(join(repo, 'testing', 'fixture', 'prototype.json'), 'utf8')).now;
  await tab.clock.install({ time: now + 60_000 });
  await tab.addInitScript((prefs) => {
    if (sessionStorage.getItem('seeded')) return;
    localStorage.setItem('cannoncommander-prefs', JSON.stringify(prefs));
    sessionStorage.setItem('seeded', '1');
  }, { language: 'en', theme, motion: 'off', ...shot.prefs });
  await tab.route('**/api/v1/**', async (route) => {
    if (route.request().resourceType() === 'eventsource') return route.continue();
    const path = new URL(route.request().url()).pathname;
    if (shot.desktop && path in ON_THE_COMPUTER) return route.fulfill({ json: ON_THE_COMPUTER[path] });
    // The fetch runs in Node, which knows nothing of the browser's mapping for the made-up host.
    const res = await route.fetch({ url: route.request().url().replace(`//${HOST}:`, '//127.0.0.1:') });
    const change = shot.api?.[path];
    if (change) {
      const body = change(await res.json().catch(() => ({})));
      return body.status ? route.fulfill({ status: body.status, json: { status: body.status } }) : route.fulfill({ response: res, json: body });
    }
    const type = res.headers()['content-type'] || '';
    if (!/json|text/.test(type)) return route.fulfill({ response: res });
    let body = await res.text();
    if (shot.desktop && path === '/api/v1/system') {
      body = JSON.stringify({ ...JSON.parse(body), deployment: 'desktop', os: 'linux' });
    }
    for (const [from, to] of ENGLISH) body = body.replaceAll(from, to);
    await route.fulfill({ response: res, body });
  });
  await tab.goto(`http://${shot.host ?? HOST}:${PORT}/`, { timeout: 240_000 });
  await tab.waitForLoadState('networkidle');
  await sleep(1500);
  await shot.prepare?.(tab);
  // The pointer of the last click would light up whatever sits under it.
  if (!shot.keepPointer) await tab.mouse.move(VIEW.width - 1, VIEW.height - 1);
  // Notes from earlier steps, such as a disk spinning up, would cover the picture.
  if (!shot.toasts) await tab.addStyleTag({ content: '.glim-toast-stack { display: none !important; }' });
  await sleep(1200);
  const file = shot.light ? `${shot.name}-${theme}.png` : `${shot.name}.png`;
  await tab.screenshot({ path: join(out, file), caret: 'hide' });
  await ctx.close();
  return file;
}

const names = process.argv.slice(2);
const shots = names.length ? SHOTS.filter((s) => names.includes(s.name)) : SHOTS;
if (shots.length < names.length) throw new Error(`no such picture: ${names.filter((n) => !SHOTS.some((s) => s.name === n)).join(', ')}`);

mkdirSync(out, { recursive: true });
const server = await fixture();
const browser = await chromium.launch({ args: ['--disable-lcd-text', `--host-resolver-rules=MAP ${HOST} 127.0.0.1`] });
let failed = 0;
try {
  for (const shot of shots) {
    for (const theme of shot.light ? ['dark', 'light'] : ['dark']) {
      try {
        console.log(`wrote ${await capture(browser, shot, theme)}`);
      } catch (e) {
        failed++;
        console.error(`${shot.name}: ${e.message.split('\n')[0]}`);
      }
    }
  }
} finally {
  await browser.close();
  server?.kill();
}
process.exitCode = failed ? 1 : 0;
