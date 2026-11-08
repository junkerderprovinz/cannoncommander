import { afterEach, describe, expect, it, vi } from 'vitest';
import { admin, me } from './admin';

const sent: { url: string; method: string; body: unknown }[] = [];
function answer(status: number, body: unknown) {
  vi.stubGlobal('fetch', vi.fn(async (url: string, init: RequestInit = {}) => {
    sent.push({ url, method: init.method ?? 'GET', body: init.body ? JSON.parse(String(init.body)) : undefined });
    return new Response(status === 204 ? null : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
  }));
}
afterEach(() => { sent.length = 0; vi.unstubAllGlobals(); });

describe('settings calls', () => {
  it('rebuild one source by its name and every source without one', async () => {
    answer(202, { id: 'j1' });
    await admin.reindex('Shares');
    await admin.reindex();
    expect(sent.map((s) => s.body)).toEqual([{ source: 'Shares' }, {}]);
  });

  it('read the index plan as times', async () => {
    answer(200, { sources: [{ source: 'Shares', path: '/mnt/user', status: 'ready', dirs: 1, files: 2, size: 3, lastIndexed: '2026-11-04T10:00:00.000Z', nextScan: '2026-11-04T10:40:00.000Z', scanEverySeconds: 2400, quickScanSeconds: 1, fullScanSeconds: 2, metadata: 0 }] });
    const [s] = await admin.index();
    expect(s!.nextScan! - s!.lastIndexed!).toBe(40 * 60000);
  });

  it('save one switch of the app settings without the rest', async () => {
    answer(200, {});
    await admin.saveApp({ previews: { thumbs: false } });
    expect(sent[0]).toMatchObject({ method: 'PUT', url: '/api/v1/admin/app', body: { previews: { thumbs: false } } });
  });

  it('give a share the address of this server and its owner', async () => {
    answer(200, [{ id: 's1', slug: 'k3x9qa-urlaub', kind: 'folder', name: 'Urlaub', paths: ['/mnt/user/fotos/Urlaub'], owner: 'anna', created: '2026-11-01T00:00:00Z', expires: null, password: false, limit: 0, downloads: 0, views: 0, uploads: false, uploadLimit: 0, uploaded: 0, notify: false, state: 'active' }]);
    const [s] = await admin.shares(true);
    expect(sent[0]!.url).toBe('/api/v1/admin/shares');
    expect(s).toMatchObject({ owner: 'anna', expires: null, url: `${location.origin}/s/k3x9qa-urlaub` });
  });

  it('take a share password off with an empty one', async () => {
    answer(200, {});
    await admin.updateShare('s1', { password: '', expires: null });
    expect(sent[0]!.body).toEqual({ password: '', expires: null });
  });

  it('send what the remote form holds in rclone terms', async () => {
    answer(201, {});
    await admin.addRemote({ name: 'nas', type: 'smb', path: 'archiv', mount: '/remotes/nas', params: { host: 'keller', pass: 'geheim' } });
    expect(sent[0]!.body).toEqual({ name: 'nas', type: 'smb', path: 'archiv', mount: '/remotes/nas', params: { host: 'keller', pass: 'geheim' } });
  });

  it('read passkeys together with why none can be added here', async () => {
    answer(200, { blocked: { title: 'Conflict', status: 409, code: 'passkeyNeedsHostName' }, items: [{ id: 'p1', name: 'Handy', origin: 'tower.lan', created: '2026-11-01T00:00:00Z', last: null }] });
    const keys = await me.passkeys();
    expect(keys.blocked?.code).toBe('passkeyNeedsHostName');
    expect(keys.items[0]!.created).toBe(Date.parse('2026-11-01T00:00:00Z'));
    answer(200, { items: [] });
    expect((await me.passkeys()).blocked).toBeNull();
  });

  it('ask for the activity of the last week in one page', async () => {
    answer(200, { items: [{ id: 3, t: '2026-11-04T10:00:00Z', who: 'jdp', what: 'login', obj: '192.168.10.16', ip: '192.168.10.16', details: {} }], total: 1, page: 1, limit: 500 });
    const [a] = await admin.activity();
    expect(sent[0]!.url).toBe('/api/v1/admin/activity?limit=500');
    expect(a).toMatchObject({ id: 3, ip: '192.168.10.16', t: Date.parse('2026-11-04T10:00:00Z') });
  });
});
