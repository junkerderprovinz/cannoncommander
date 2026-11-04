// @vitest-environment node
// The tools part of the fixture answers in the shapes of the contracts in backend/internal/{tools,fileops,shares,prefs},
// and its jobs do to the tree what the server's would.
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { opsFixture } from '../../scripts/fixture-ops.js';
import { toolsFixture } from '../../scripts/fixture-tools.js';
import { fixtureMiddleware } from '../../scripts/fixture.js';

let server: Server, base = '';
beforeAll(async () => {
  const ops = opsFixture(undefined, [toolsFixture]), plain = fixtureMiddleware();
  server = createServer((req, res) => ops(req as never, res as never, () => plain(req as never, res as never, () => { res.statusCode = 404; res.end(); })));
  await new Promise<void>((r) => server.listen(0, '127.0.0.1', () => r()));
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}/api/v1`;
});
afterAll(() => server.close());

type Body = any;
const api = async (method: string, path: string, body?: unknown, headers: Record<string, string> = {}) => {
  const res = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json', ...headers }, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await res.text();
  return { status: res.status, body: (text ? JSON.parse(text) : null) as Body, headers: res.headers };
};
const names = async (path: string) => ((await api('GET', '/fs/list?path=' + encodeURIComponent(path))).body.items as { name: string }[]).map((k) => k.name);
const item = async (dir: string, name: string) => ((await api('GET', '/fs/list?path=' + encodeURIComponent(dir))).body.items as { name: string; mode: string; link?: string }[]).find((k) => k.name === name);
async function run(kind: string, params: unknown) {
  const started = await api('POST', '/jobs', { kind, params });
  if (started.status !== 202) return started;
  for (let i = 0; i < 400; i++) {
    const job = ((await api('GET', '/jobs')).body.jobs as { id: string; state: string }[]).find((j) => j.id === started.body.id)!;
    if (!['queued', 'running', 'paused', 'asking'].includes(job.state)) return { status: 200, body: job as Body, headers: started.headers };
    await new Promise((r) => setTimeout(r, 50));
  }
  throw new Error('the job did not end');
}

describe('batch rename', () => {
  it('renames for real only without errors, and the undo step takes it back', async () => {
    const paths = ['/mnt/user/dokumente/notizen.txt', '/mnt/user/dokumente/Einkaufsliste.md'];
    const clash = await api('POST', '/fs/rename/batch', { paths, names: ['Mietvertrag.pdf', 'liste.md'] });
    expect(clash.status).toBe(409);
    expect(clash.body.rows.map((r: { error?: string }) => r.error)).toEqual(['exists', undefined]);
    const done = await api('POST', '/fs/rename/batch', { paths, names: ['notiz.txt', 'liste.md'] });
    expect(done.body).toMatchObject({ changed: 2, errors: 0, done: true });
    expect(await names('/mnt/user/dokumente')).toEqual(expect.arrayContaining(['notiz.txt', 'liste.md']));
    expect((await api('POST', '/undo', { id: done.body.undo })).status).toBe(200);
    expect(await names('/mnt/user/dokumente')).toEqual(expect.arrayContaining(['notizen.txt', 'Einkaufsliste.md']));
  });
});

describe('links and rights', () => {
  it('makes a symbolic link that may point nowhere, and a hard one only to a file', async () => {
    expect((await api('POST', '/fs/link', { target: '/mnt/user/gibts/nicht.txt', path: '/mnt/user/downloads/leer.lnk' })).status).toBe(201);
    expect((await item('/mnt/user/downloads', 'leer.lnk'))?.link).toBe('/mnt/user/gibts/nicht.txt');
    expect((await api('POST', '/fs/link', { target: '/mnt/user/downloads/album.flac', path: '/mnt/user/downloads/leer.lnk' })).status).toBe(409);
    expect((await api('POST', '/fs/link', { target: '/mnt/user/fotos', path: '/mnt/user/downloads/fotos', hard: true })).status).toBe(404);
    expect((await api('POST', '/fs/link', { target: '/mnt/user/downloads/album.flac', path: '/mnt/user/backups/album.flac', hard: true })).status).toBe(201);
    expect((await item('/mnt/user/backups', 'album.flac'))?.link).toBeUndefined();
  });

  it('sets rights at once, and for everything inside as a job', async () => {
    expect((await api('PATCH', '/fs/attrs', { paths: ['/mnt/user/fotos/2025'], mode: '9' })).status).toBe(400);
    expect((await api('PATCH', '/fs/attrs', { paths: ['/mnt/user/fotos/2025'], mode: '0750' })).body).toEqual({ changed: 1 });
    expect((await item('/mnt/user/fotos', '2025'))?.mode).toBe('0750');
    const job = await api('PATCH', '/fs/attrs', { paths: ['/mnt/user/fotos/2025'], mode: '0770', recursive: true });
    expect(job.status).toBe(202);
    expect(job.body.kind).toBe('attrsRecursive');
  });
});

describe('archives', { timeout: 30000 }, () => {
  it('packs into a numbered name when the name is taken, and tests a damaged backup', async () => {
    const one = await run('pack', { sources: ['/mnt/user/dokumente/Mietvertrag.pdf'], dest: '/mnt/user/backups', name: 'akten', format: 'zip' });
    const two = await run('pack', { sources: ['/mnt/user/dokumente/Mietvertrag.pdf'], dest: '/mnt/user/backups', name: 'akten', format: 'zip' });
    expect([one.body.result.path, two.body.result.path]).toEqual(['/mnt/user/backups/akten.zip', '/mnt/user/backups/akten (1).zip']);
    expect((await api('POST', '/jobs', { kind: 'pack', params: { sources: ['/mnt/user/dokumente/Mietvertrag.pdf'], dest: '/mnt/user/backups', name: 'x', format: 'tar.zst', password: 'geheim' } })).status).toBe(400);
    const tested = await run('testArchive', { paths: ['/mnt/user/backups/appdata-2026-10-24.tar.zst', '/mnt/user/backups/akten.zip'] });
    expect(tested.body.result.archives.map((a: { ok: boolean; damage?: { entry: string } }) => [a.ok, a.damage?.entry])).toEqual([[false, 'appdata/bombvault/bombvault.db'], [true, undefined]]);
  });

  it('tells what an archive holds and unpacks it into a folder of its own', async () => {
    const info = await api('GET', '/tools/archive?path=' + encodeURIComponent('/mnt/user/downloads/Fotos-Urlaub-2026.zip'));
    expect(info.body).toMatchObject({ format: 'zip', encrypted: false });
    expect(info.body.entries).toBeGreaterThan(0);
    const done = await run('unpack', { archives: ['/mnt/user/downloads/Fotos-Urlaub-2026.zip'], dest: '/mnt/user/backups', own: true, clash: 'both' });
    expect(done.body.result).toEqual({ paths: ['/mnt/user/backups/Fotos-Urlaub-2026'], skipped: 0 });
  });
});

describe('checksums and parts', { timeout: 30000 }, () => {
  it('reads both list styles and finds the file that differs', async () => {
    const lines = (await api('POST', '/tools/checksum/parse', { text: `${'a'.repeat(64)}  eins.iso\nSHA1 (zwei.iso) = ${'b'.repeat(40)}\nunsinn` })).body.lines;
    expect(lines).toEqual([{ name: 'eins.iso', hash: 'a'.repeat(64), algo: 'sha256' }, { name: 'zwei.iso', hash: 'b'.repeat(40), algo: 'sha1' }, { name: '', bad: 'unsinn' }]);
    const verified = await run('verify', { file: '/mnt/user/isos/checksums.sha256' });
    expect(verified.body.result.lines.map((l: { state: string }) => l.state)).toEqual(['ok', 'mismatch', 'ok']);
  });

  it('hashes files and writes the list beside them', async () => {
    const done = await run('checksum', { sources: ['/mnt/user/isos/Win11_24H2_German_x64.iso'], algo: 'md5', save: '/mnt/user/isos/win.md5' });
    expect(done.body.result.files[0]).toMatchObject({ name: 'Win11_24H2_German_x64.iso' });
    expect(done.body.result.files[0].hash).toMatch(/^[0-9a-f]{32}$/);
    expect(await names('/mnt/user/isos')).toContain('win.md5');
  });

  it('splits a file and combines the parts, checked against their sum', async () => {
    const split = await run('split', { path: '/mnt/user/downloads/Big.Buck.Bunny.2008.1080p.mkv', dest: '/mnt/user/backups', partSize: 512 * 1024 * 1024, sum: true });
    expect(split.body.result.first).toBe('/mnt/user/backups/Big.Buck.Bunny.2008.1080p.mkv.001');
    const combined = await run('combine', { path: '/mnt/user/backups/Big.Buck.Bunny.2008.1080p.mkv.002', dest: '/mnt/user/backups', check: true, drop: true });
    expect(combined.body.result).toEqual({ path: '/mnt/user/backups/Big.Buck.Bunny.2008.1080p.mkv', checked: true });
    expect((await names('/mnt/user/backups')).some((n) => n.endsWith('.mkv.001'))).toBe(false);
  });
});

describe('clean up and loading', { timeout: 30000 }, () => {
  it('finds the leftovers by group, with the server\'s notes', async () => {
    const scan = (await api('POST', '/tools/cleanup/scan', { path: '/mnt/user/downloads/unfertig' })).body;
    expect(scan.groups.temp.map((f: { path: string }) => f.path.split('/').pop())).toEqual(expect.arrayContaining(['.DS_Store', 'Thumbs.db']));
    expect(scan.groups.ext).toEqual([expect.objectContaining({ note: 'png', to: 'Urlaub-007.png' })]);
    expect(scan.groups.name.map((f: { note: string }) => f.note)).toEqual(expect.arrayContaining(['edgeSpace', 'windowsChars']));
    expect(scan.groups.link[0].note).toBe('/mnt/user/isos/Win10_22H2_German_x64.iso');
  });

  it('refuses addresses in the local network and loads the others as a job', async () => {
    expect((await api('POST', '/jobs', { kind: 'fetchUrl', params: { url: 'http://192.168.10.10/x.iso', dest: '/mnt/user/downloads' } })).status).toBe(400);
    const done = await run('fetchUrl', { url: 'https://example.org/a/b/neu.txt', dest: '/mnt/user/downloads' });
    expect(done.body.result.path).toBe('/mnt/user/downloads/neu.txt');
  });
});

describe('share links', () => {
  it('creates a link with its QR code, keeps addresses unique and rolls a new one', async () => {
    const made = await api('POST', '/shares', { paths: ['/mnt/user/dokumente/Mietvertrag.pdf'], slug: 'miete-2026', password: 'geheim123', notify: false });
    expect(made.status).toBe(201);
    expect(made.body).toMatchObject({ kind: 'file', name: 'Mietvertrag.pdf', password: true, state: 'active', slug: 'miete-2026' });
    expect((await api('POST', '/shares', { paths: ['/mnt/user/dokumente'], slug: 'miete-2026' })).status).toBe(409);
    expect((await api('POST', '/shares', { paths: ['/mnt/user/dokumente', '/mnt/user/isos'] })).status).toBe(400);
    const qr = (await api('GET', `/shares/${made.body.id}/qr?origin=${encodeURIComponent('https://files.example.org')}`)).body;
    expect(qr.url).toBe('https://files.example.org/s/miete-2026');
    expect(qr.qr).toMatch(/^<svg/);
    const rolled = await api('PATCH', `/shares/${made.body.id}`, { reroll: true });
    expect(rolled.body.slug).not.toBe('miete-2026');
    expect(((await api('GET', '/shares')).body as { id: string }[]).some((s) => s.id === made.body.id)).toBe(true);
    expect((await api('DELETE', `/shares/${made.body.id}`)).status).toBe(204);
  });

  it('keeps own rename presets with unique names', async () => {
    expect((await api('PUT', '/me/rename-presets', { items: [{ name: 'a', options: {} }, { name: 'a', options: {} }] })).status).toBe(409);
    expect((await api('PUT', '/me/rename-presets', { items: [{ name: 'Fotos', options: { name: '[A]' } }] })).status).toBe(204);
    expect((await api('GET', '/me/rename-presets')).body).toEqual({ items: [{ name: 'Fotos', options: { name: '[A]' } }] });
  });

  it('sends a folder as one zip to save', async () => {
    const res = await fetch(base + '/fs/archive-stream', { method: 'POST', headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, body: 'paths=%2Fmnt%2Fuser%2Fdokumente&format=zip&name=akten.zip' });
    expect(res.status).toBe(200);
    expect(res.headers.get('content-disposition')).toContain('akten.zip');
  });
});

describe('where on Unraid', () => {
  it('names the disks a path lies on and the other names of a hard link', async () => {
    const where = (await api('GET', '/unraid/where?path=' + encodeURIComponent('/mnt/user/downloads/album.flac'))).body;
    expect(where).toMatchObject({ volume: 'user', share: 'downloads', dir: false, files: 1 });
    expect(where.links.count).toBe(where.links.paths.length + 1);
  });
});
