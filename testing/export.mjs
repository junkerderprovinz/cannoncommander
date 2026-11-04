// node testing/export.mjs [out.json]: opens the built prototype and writes the data every screen draws from as JSON,
// so the generator can build the same tree on disk and the tests can check the app against the same numbers.
import { existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { pathToFileURL } from 'node:url';

const repo = join(import.meta.dirname, '..');
const page = join(repo, 'prototype', 'dist', 'cannoncommander.html');
const out = process.argv[2] || join(import.meta.dirname, 'fixture', 'prototype.json');

// Playwright lives with the prototype, which keeps the export runnable before the new frontend has its own packages.
const { chromium } = createRequire(join(repo, 'prototype', 'package.json'))('playwright');

if (!existsSync(page)) execFileSync(process.execPath, [join(repo, 'prototype', 'build.mjs')], { stdio: 'inherit' });

const browser = await chromium.launch();
const tab = await browser.newPage();
const errors = [];
tab.on('pageerror', (e) => errors.push(String(e)));
await tab.clock.install({ time: Date.UTC(2026, 10, 2, 9, 30) });
await tab.goto(pathToFileURL(page).href);
await tab.waitForFunction(() => typeof CC !== 'undefined' && CC.unraid && CC.ck);

const data = await tab.evaluate(() => {
  const textKinds = new Set(['text', 'code', 'config', 'log', 'subtitle', 'playlist', 'checksum', 'key', 'calendar', 'mail', 'weblink']);
  const formatOf = Object.fromEntries(FORMATS.flatMap((f) => f.ext.map((e) => [e, f.id])));
  const plain = (v) => JSON.parse(JSON.stringify(v));

  // Column values the list shows for a file, so metadata the generator writes can follow them.
  const columns = (node, path) => {
    const k = { ...node, kind: FS.kindOf(node), path };
    const out = {};
    for (const c of COLUMNS) {
      if (!c.value || ['name', 'size', 'modified'].includes(c.id)) continue;
      try {
        const v = c.value(k, FS.parent(path));
        if (v !== '' && v != null) out[c.id] = String(v).replace(/<[^>]+>/g, '');
      } catch {
        // A column that needs list state the export does not have stays out.
      }
    }
    return out;
  };

  const node = (n, path) => {
    const { children, zip, ...own } = n;
    const out = plain(own);
    if (!n.dir) {
      out.format = formatOf[FS.ext(n.name)] || '';
      out.view = CC.ck.viewKind(n);
      if (CC.ck.editable(n) || textKinds.has(out.format)) out.text = CC.ck.textFor(n);
      out.cols = columns(n, path);
    }
    if (zip) out.zip = zip.map((z) => node(z, `${path}::/${z.name}`));
    if (children) out.children = children.map((c) => node(c, FS.join(path, c.name)));
    return out;
  };

  const u = CC.unraid;
  return {
    now: NOW,
    tree: node(FS.root, '/'),
    volumes: { free: FS.FREE, total: FS.TOTAL, label: FS.LABEL },
    unraid: { disks: u.DISKS, devs: u.DEVS, shares: plain(u.SHARES), spin: plain(u.spin) },
    data: plain(DATA),
  };
});

await browser.close();
if (errors.length) {
  console.error(errors.join('\n'));
  process.exit(1);
}
mkdirSync(dirname(out), { recursive: true });
writeFileSync(out, JSON.stringify(data, null, 1) + '\n');
let files = 0;
const count = (n) => { if (!n.dir) files++; (n.children || []).forEach(count); };
count(data.tree);
console.log(`${out}: ${files} files, ${data.unraid.devs.length} devices, ${Object.keys(data.data).length} data sets`);
