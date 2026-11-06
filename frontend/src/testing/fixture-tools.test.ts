// @vitest-environment node
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { opsFixture } from '../../scripts/fixture-ops.js';
import { toolsFixture } from '../../scripts/fixture-tools.js';
import { createFixture, fixtureMiddleware } from '../../scripts/fixture.js';

let server: Server, base = '';
beforeAll(async () => {
  const ops = opsFixture(undefined, [toolsFixture]), plain = fixtureMiddleware();
  server = createServer((req, res) => ops(req as never, res as never, () => plain(req as never, res as never, () => { res.statusCode = 404; res.end(); })));
  await new Promise<void>((r) => server.listen(0, '127.0.0.1', () => r()));
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}/api/v1`;
});
afterAll(() => server.close());

const api = async (method: string, path: string, body?: unknown) => {
  const res = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
  return { status: res.status, body: res.status === 204 ? null : await res.json() };
};
const done = async (id: string) => {
  for (let i = 0; i < 200; i++) {
    const j = (await api('GET', '/jobs/' + id)).body;
    if (!['queued', 'running'].includes(j.state)) return j;
    await new Promise((r) => setTimeout(r, 50));
  }
  throw new Error('job did not end');
};
const L = '/mnt/user/dokumente', R = '/mnt/user/backups/dokumente';

describe('fixture search', () => {
  const fx = createFixture();
  it('finds by pattern, field and tag, folders first', () => {
    expect(fx.search('/mnt/user/downloads', { q: '*.iso' })!.items.map((h) => h.name)).toEqual(['debian-13.1.0-amd64-netinst.iso', 'ubuntu-24.04.3-desktop-amd64.iso']);
    expect(fx.search('/mnt/user', { q: 'jahr:>2000 *.mp3' })!.total).toBe(2);
    expect(fx.search('/mnt/user', { q: '#steuer' })!.items.map((h) => h.name)).toEqual(['Rechnung-Oktober.pdf', 'Steuer 2025.xlsx']);
  });
  it('reads the codes the interface sends for words, as the server does', () => {
    const total = (q: string) => fx.search('/mnt/user', { q })!.total;
    for (const [code, word] of [['kind:music', 'art:Musik'], ['type:img', 'typ:Bild'], ['type:folder', 'typ:Ordner'], ['encrypted:no', 'verschlüsselt:Nein']]) {
      expect(total(code), code).toBeGreaterThan(0);
      expect(total(code), code).toBe(total(word));
    }
  });
  it('names unknown fields and finds nothing then', () => {
    const got = fx.search('/mnt/user', { q: 'farbtiefe:>8 foo:1' })!;
    expect(got.unknown).toEqual(['foo']);
    expect(got.items).toEqual([]);
  });
  it('counts every hit and hands out the first ones', () => {
    const got = fx.search('/mnt/user', { q: '', when: '30', limit: 5 })!;
    expect(got.items).toHaveLength(5);
    expect(got.total).toBeGreaterThan(5);
    expect(got.more).toBe(true);
  });
  it('reads a size alone in MB', () => {
    const big = fx.search('/mnt/user', { q: '*.iso *.img', min: '1 GB' })!.items;
    expect(big.every((h) => h.size >= 1024 ** 3)).toBe(true);
    expect(fx.search('/mnt/user', { q: '*.iso', max: '800' })!.items.map((h) => h.name)).toEqual(['debian-13.1.0-amd64-netinst.iso', 'debian-13.1.0-amd64-netinst.iso']);
  });
});

describe('fixture tools', () => {
  it('keeps saved searches, the prototype two first', async () => {
    expect((await api('GET', '/search/saved')).body.items.map((s: { name: string }) => s.name)).toEqual(['Große Abbilder', 'Diesen Monat geändert']);
    const made = (await api('POST', '/search/saved', { name: 'Filme', query: { name: '*.mkv', in: '/mnt/user' } })).body;
    expect(made.id).toBe(3);
    expect((await api('PUT', `/search/saved/${made.id}`, { name: 'Alle Filme', query: made.query })).body.name).toBe('Alle Filme');
    expect((await api('POST', '/search/saved', { name: '', query: {} })).status).toBe(400);
    expect((await api('DELETE', `/search/saved/${made.id}`)).status).toBe(204);
    expect((await api('DELETE', `/search/saved/${made.id}`)).status).toBe(404);
  });

  it('compares two folders and counts the states', async () => {
    const got = (await api('POST', '/tools/compare', { left: L, right: R, top: true })).body;
    expect(got.counts).toEqual({ onlyL: 3, onlyR: 1, newerR: 2, same: 4, newerL: 1 });
    expect(got.items.find((i: { name: string }) => i.name === 'Bewerbung.docx')).toMatchObject({ state: 'newerR', left: { dir: false }, right: { dir: false } });
  });

  it('plans a sync with the newer target switched off, and mirrors only one way', async () => {
    const plan = (await api('POST', '/tools/sync/plan', { left: L, right: R, direction: 'right', recursive: true })).body.steps;
    expect(plan.find((s: { rel: string }) => s.rel === 'Bewerbung.docx')).toMatchObject({ act: 'replace', to: 'R', off: true, why: 'rightNewer' });
    const mirror = (await api('POST', '/tools/sync/plan', { left: L, right: R, direction: 'right', mirror: true })).body.steps;
    expect(mirror.some((s: { act: string }) => s.act === 'delete')).toBe(true);
    const both = (await api('POST', '/tools/sync/plan', { left: L, right: R, direction: 'both', mirror: true })).body.steps;
    expect(both.some((s: { act: string }) => s.act === 'delete')).toBe(false);
  });

  it('runs a sync as a job and leaves the folders alike', async () => {
    const steps = (await api('POST', '/tools/sync/plan', { left: L, right: R, direction: 'right' })).body.steps.filter((s: { off?: boolean }) => !s.off);
    const job = (await api('POST', '/jobs', { kind: 'sync', params: { left: L, right: R, steps } })).body;
    expect(job).toMatchObject({ kind: 'sync', name: 'dokumente', dest: R, count: steps.length });
    expect((await done(job.id)).state).toBe('done');
    const after = (await api('POST', '/tools/compare', { left: L, right: R, top: true })).body.counts;
    expect(after.onlyL).toBeUndefined();
    expect(after.newerL).toBeUndefined();
  });

  it('compares two files line by line', async () => {
    const d = (await api('POST', '/tools/diff', { left: L + '/docker-compose.yml', right: R + '/docker-compose.yml' })).body;
    expect(d.binary).toBe(false);
    expect(d.hunks).toBeGreaterThan(0);
    expect(d.rows.every((r: { t: string }) => ['eq', 'chg', 'del', 'add'].includes(r.t))).toBe(true);
    expect((await api('POST', '/tools/diff', { left: L, right: R })).status).toBe(404);
  });

  it('finds duplicates as a job, the newest copy first', async () => {
    const job = (await api('POST', '/tools/duplicates', { path: '/mnt/user', by: 'name' })).body;
    expect(job.kind).toBe('duplicates');
    const { result } = await done(job.id);
    const iso = result.groups.find((g: { name: string }) => g.name === 'debian-13.1.0-amd64-netinst.iso');
    expect(iso.copies.map((c: { at: string }) => c.at)).toEqual(['/mnt/user/downloads', '/mnt/user/isos']);
    expect(iso.wasted).toBe(iso.size);
    expect(result.wasted).toBe(result.groups.reduce((a: number, g: { wasted: number }) => a + g.wasted, 0));
  });

  it('measures a folder for the treemap', async () => {
    const u = (await api('GET', '/tools/diskusage?path=/mnt/user/backups')).body;
    expect(u.items.map((i: { size: number }) => i.size)).toEqual([...u.items.map((i: { size: number }) => i.size)].sort((a, b) => b - a));
    expect(u.size).toBe(u.items.reduce((a: number, i: { size: number }) => a + i.size, 0));
    const disks = (await api('GET', '/tools/diskusage?path=/mnt/user/backups&kind=disk')).body.largest;
    expect(disks.map((f: { name: string }) => f.name)).toEqual(['vm-win11-2026-10-01.img', 'ubuntu-24.04.3-desktop-amd64.iso']);
    expect((await api('GET', '/tools/diskusage?path=/mnt/user/gibtsnicht')).status).toBe(404);
  });

  it('searches inside files as a job and names the line', async () => {
    const job = (await api('POST', '/jobs', { kind: 'contentSearch', params: { path: '/mnt/user', text: 'PLEX' } })).body;
    const { result } = await done(job.id);
    expect(result.total).toBeGreaterThan(0);
    expect(result.items.every((h: { line: number }) => h.line > 0)).toBe(true);
    expect((await api('POST', '/jobs', { kind: 'contentSearch', params: { path: '/mnt/user', text: ' ' } })).status).toBe(400);
  });

  it('hands a sync to ArrowLoop', async () => {
    const got = await api('POST', '/tools/arrowloop', { name: 'Nachts', left: L, right: R, direction: 'right', every: 'daily', at: '03:00' });
    expect(got.status).toBe(201);
    expect(got.body.url).toMatch(/^http/);
  });
});
