// @vitest-environment node
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { opsFixture } from '../../scripts/fixture-ops.js';
import { fixtureMiddleware } from '../../scripts/fixture.js';

let server: Server, base = '';
const events: { type: string; data: Record<string, unknown> }[] = [];
let stream: AbortController;
beforeAll(async () => {
  const ops = opsFixture(), plain = fixtureMiddleware();
  server = createServer((req, res) => ops(req as never, res as never, () => plain(req as never, res as never, () => { res.statusCode = 404; res.end(); })));
  await new Promise<void>((r) => server.listen(0, '127.0.0.1', () => r()));
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}/api/v1`;
  stream = new AbortController();
  const res = await fetch(base + '/events', { signal: stream.signal });
  void (async () => {
    const reader = res.body!.getReader(), text = new TextDecoder();
    let buf = '';
    for (;;) {
      const { value, done } = await reader.read().catch(() => ({ value: undefined, done: true }));
      if (done) return;
      buf += text.decode(value);
      const frames = buf.split('\n\n');
      buf = frames.pop()!;
      for (const f of frames) {
        const data = f.split('\n').find((l) => l.startsWith('data: '));
        if (data) events.push(JSON.parse(data.slice(6)));
      }
    }
  })();
});
afterAll(() => { stream.abort(); server.close(); });

const api = async (method: string, path: string, body?: unknown) => {
  const res = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
  return { status: res.status, body: res.status === 204 ? null : await res.json() };
};
const until = async (test: () => boolean | Promise<boolean>, tries = 200) => {
  for (let i = 0; i < tries; i++) { if (await test()) return; await new Promise((r) => setTimeout(r, 50)); }
  throw new Error('condition not met');
};

describe('fixture unraid', () => {
  it('answers the state in the shape of the contract', async () => {
    const st = (await api('GET', '/unraid/state')).body;
    expect(st.available).toBe(true);
    expect(st.mover).toEqual({ schedule: '40 3 * * *', active: false });
    expect(st.disks.filter((d: { kind: string }) => d.kind === 'data').map((d: { name: string; spundown: boolean }) => `${d.name}:${d.spundown}`))
      .toEqual(['disk1:true', 'disk2:false', 'disk3:true']);
    const downloads = st.shares.find((s: { name: string }) => s.name === 'downloads');
    expect(downloads).toMatchObject({ primary: { type: 'pool', name: 'cache' }, secondary: { type: 'array' }, mover: 'toSecondary', disks: ['disk1'] });
    expect(st.shares.find((s: { name: string }) => s.name === 'filme')).toMatchObject({ primary: { type: 'array' }, disks: ['disk1', 'disk2', 'disk3'] });
    expect(st.shares.find((s: { name: string }) => s.name === 'filme').secondary).toBeUndefined();
  });

  it('tells where a folder lies and which other names a hard link has', async () => {
    const folder = (await api('GET', '/unraid/where?path=' + encodeURIComponent('/mnt/user/downloads'))).body;
    expect(folder.dir).toBe(true);
    expect(folder.on.map((o: { device: string }) => o.device)).toEqual(['cache', 'disk1']);
    const cache = folder.on.find((o: { device: string }) => o.device === 'cache');
    expect(cache.files).toBe(19);
    const film = (await api('GET', '/unraid/where?path=' + encodeURIComponent('/mnt/user/downloads/Big.Buck.Bunny.2008.1080p.mkv'))).body;
    expect(film.on).toEqual([expect.objectContaining({ device: 'disk1', files: 1, asleep: true })]);
    expect(film.links).toEqual({ count: 2, paths: ['/mnt/user/filme/Big Buck Bunny (2008)/Big Buck Bunny (2008).mkv'], complete: true });
  });

  it('fills the disk column from where the files really lie', async () => {
    const got = (await api('POST', '/fs/meta', { paths: ['/mnt/user/downloads/album.flac', '/mnt/user/downloads', '/mnt/disk2/filme'], columns: ['disk'] })).body.items;
    expect(got['/mnt/user/downloads/album.flac'].disk).toBe('cache');
    expect(got['/mnt/user/downloads'].disk).toBe('cache, disk1');
    expect(got['/mnt/disk2/filme'].disk).toBe('disk2');
  });

  it('counts what the rights repair changes and leaves appdata alone', async () => {
    const blocked = (await api('POST', '/unraid/fixperms/scan', { path: '/mnt/user/appdata/plex' })).body;
    expect(blocked.blocked).toBe('appdata');
    const scan = (await api('POST', '/unraid/fixperms/scan', { path: '/mnt/user' })).body;
    expect(scan.kept.map((k: { path: string }) => k.path)).toEqual(['/mnt/user/appdata', '/mnt/user/system']);
    expect(scan.files).toBeGreaterThan(50);
    const job = (await api('POST', '/jobs', { kind: 'fixPerms', params: { path: '/mnt/user/downloads' } })).body;
    expect(job.kind).toBe('fixPerms');
    await until(async () => ((await api('GET', '/jobs')).body.jobs as { id: string; state: string }[]).find((j) => j.id === job.id)!.state === 'done');
    const done = ((await api('GET', '/jobs')).body.jobs as { id: string; result: { entries: number } }[]).find((j) => j.id === job.id)!;
    expect(done.result.entries).toBeGreaterThan(10);
    const after = (await api('POST', '/unraid/fixperms/scan', { path: '/mnt/user/downloads' })).body;
    expect(after.wrong).toBe(0);
  });

  it('sends disks to sleep, wakes one on a read and reports both', async () => {
    expect((await api('POST', '/unraid/spindown', {})).body.disks).toEqual(['disk1', 'disk2', 'disk3']);
    await until(() => events.some((e) => e.type === 'unraid.spin' && (e.data.changed as string[]).includes('disk2')));
    expect((await api('POST', '/unraid/spindown', { disks: ['disk9'] })).status).toBe(400);
    await fetch(base + '/fs/raw?path=' + encodeURIComponent('/mnt/user/filme/Sintel (2010)/Sintel (2010).mkv'), { headers: { Range: 'bytes=0-15' } });
    await until(() => events.some((e) => e.type === 'unraid.spin' && !(e.data.asleep as string[]).includes('disk2')));
    await api('PUT', '/fixture/spin', { disk: 'disk2', asleep: true });
    const st = (await api('GET', '/unraid/state')).body;
    expect(st.disks.find((d: { name: string }) => d.name === 'disk2').spundown).toBe(true);
  });

  it('plays a parity check that holds big copies and lets them go when it ends', async () => {
    await api('PUT', '/fixture/parity', { busy: true });
    expect((await api('GET', '/unraid/state')).body.parity).toMatchObject({ running: true, kind: 'check' });
    expect((await api('POST', '/ops/preflight', { kind: 'copy', sources: ['/mnt/user/downloads/setup-notes.md'], dest: '/mnt/user/dokumente' })).body.parityBusy).toBe(true);
    await until(() => events.some((e) => e.type === 'unraid.parity' && (e.data.percent as number) > 0), 100);
    await api('PUT', '/fixture/parity', { busy: false, finished: true });
    const parity = (await api('GET', '/unraid/state')).body.parity;
    expect(parity).toMatchObject({ running: false, lastExit: 0 });
    expect(events.filter((e) => e.type === 'unraid.parity').at(-1)!.data.running).toBe(false);
  });
});
