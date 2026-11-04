// @vitest-environment node
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { opsFixture } from '../../scripts/fixture-ops.js';
import { fixtureMiddleware } from '../../scripts/fixture.js';

let server: Server, base = '';
beforeAll(async () => {
  const ops = opsFixture(), plain = fixtureMiddleware();
  server = createServer((req, res) => ops(req as never, res as never, () => plain(req as never, res as never, () => { res.statusCode = 404; res.end(); })));
  await new Promise<void>((r) => server.listen(0, '127.0.0.1', () => r()));
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}/api/v1`;
});
afterAll(() => server.close());

const api = async (method: string, path: string, body?: unknown, headers: Record<string, string> = {}) => {
  const res = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json', ...headers }, body: body instanceof Blob ? body : body === undefined ? undefined : JSON.stringify(body) });
  return { status: res.status, body: res.status === 204 ? null : await res.json() };
};
const names = async (path: string) => ((await api('GET', '/fs/list?path=' + encodeURIComponent(path))).body.items as { name: string }[]).map((k) => k.name);
const until = async (test: () => Promise<boolean>) => {
  for (let i = 0; i < 200; i++) { if (await test()) return; await new Promise((r) => setTimeout(r, 50)); }
  throw new Error('condition not met');
};
const jobOf = async (id: string) => ((await api('GET', '/jobs')).body.jobs as { id: string; state: string; undo?: string; conflict?: { key: string } }[]).find((j) => j.id === id)!;

describe('fixture jobs', () => {
  it('finds the names that exist at the target before a copy', async () => {
    const pf = (await api('POST', '/ops/preflight', { kind: 'copy', sources: ['/mnt/user/downloads/setup-notes.md'], dest: '/mnt/user/dokumente' })).body;
    expect(pf.count).toBe(1);
    expect(pf.conflicts).toEqual([]);
    await api('POST', '/fs/touch', { path: '/mnt/user/dokumente/setup-notes.md' });
    const again = (await api('POST', '/ops/preflight', { kind: 'copy', sources: ['/mnt/user/downloads/setup-notes.md'], dest: '/mnt/user/dokumente' })).body;
    expect(again.conflicts.map((c: { key: string }) => c.key)).toEqual(['setup-notes.md']);
  });

  it('runs a copy that asks about a name on its way, then finishes with an undo step', async () => {
    const started = (await api('POST', '/jobs', { kind: 'copy', params: { sources: ['/mnt/user/downloads/setup-notes.md'], dest: '/mnt/user/dokumente', rule: 'ask' } })).body;
    await until(async () => (await jobOf(started.id)).state === 'asking');
    expect((await jobOf(started.id)).conflict?.key).toBe('setup-notes.md');
    await api('POST', `/jobs/${started.id}/decide`, { choice: 'both' });
    await until(async () => (await jobOf(started.id)).state === 'done');
    expect(await names('/mnt/user/dokumente')).toEqual(expect.arrayContaining(['setup-notes.md', 'setup-notes (1).md']));
    expect((await jobOf(started.id)).undo).toBeTruthy();
  });

  it('puts things in the trash, restores them and empties it', async () => {
    const trashed = (await api('POST', '/jobs', { kind: 'trash', params: { paths: ['/mnt/user/dokumente/setup-notes (1).md'] } })).body;
    await until(async () => (await jobOf(trashed.id)).state === 'done');
    const list = (await api('GET', '/trash')).body.items as { id: string; name: string }[];
    const entry = list.find((e) => e.name === 'setup-notes (1).md')!;
    expect(await names('/mnt/user/dokumente')).not.toContain('setup-notes (1).md');
    const back = (await api('POST', '/trash/restore', { ids: [entry.id] })).body;
    await until(async () => (await jobOf(back.id)).state === 'done');
    expect(await names('/mnt/user/dokumente')).toContain('setup-notes (1).md');
    const empty = (await api('POST', '/trash/empty')).body;
    await until(async () => (await jobOf(empty.id)).state === 'done');
    expect((await api('GET', '/trash')).body.count).toBe(0);
  });

  it('takes back a new folder at once', async () => {
    const made = (await api('POST', '/fs/mkdir', { path: '/mnt/user/dokumente/Neu' })).body;
    expect(await names('/mnt/user/dokumente')).toContain('Neu');
    expect((await api('POST', '/undo', { id: made.undo })).status).toBe(200);
    expect(await names('/mnt/user/dokumente')).not.toContain('Neu');
  });

  it('takes an upload in chunks and asks first about a name that exists', async () => {
    const clash = await api('POST', '/uploads', { path: '/mnt/user/downloads/setup-notes.md', size: 3 });
    expect(clash.status).toBe(409);
    expect(clash.body.conflict.key).toBe('setup-notes.md');
    const up = (await api('POST', '/uploads', { path: '/mnt/user/downloads/neu.bin', size: 600 * 1024 })).body;
    let state = up;
    while (state.state === 'open') {
      const chunk = new Blob([new Uint8Array(Math.min(state.chunk, state.size - state.offset))]);
      state = (await api('PUT', `/uploads/${up.id}`, chunk, { 'Content-Type': 'application/octet-stream', 'Upload-Offset': String(state.offset) })).body;
    }
    expect(state).toMatchObject({ state: 'done', offset: 600 * 1024 });
    expect(await names('/mnt/user/downloads')).toContain('neu.bin');
  });

  it('plans gathering a folder onto the disk that has to copy least', async () => {
    const plan = (await api('POST', '/unraid/gather/plan', { path: '/mnt/user/filme', mode: 'gather' })).body;
    expect(plan.disk).toMatch(/^disk\d$/);
    expect(plan.devices.map((d: { device: string }) => d.device)).toEqual(['cache', 'disk1', 'disk2', 'disk3']);
    expect((await api('POST', '/unraid/gather/plan', { path: '/mnt/cache', mode: 'gather' })).status).toBe(400);
  });
});
