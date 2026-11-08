// @vitest-environment node
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { authFixtureMiddleware } from '../../scripts/fixture-auth.js';

let server: Server, base = '';
beforeAll(async () => {
  const auth = authFixtureMiddleware();
  server = createServer((req, res) => auth(req as never, res as never, () => { res.statusCode = 200; res.setHeader('Content-Type', 'application/json'); res.end('{"name":"jdp"}'); }));
  await new Promise<void>((r) => server.listen(0, '127.0.0.1', () => r()));
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}/api/v1`;
});
afterAll(() => server.close());

const api = async (method: string, path: string, body?: unknown, headers: Record<string, string> = {}) => {
  const res = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json', ...headers }, body: body instanceof Blob ? body : body === undefined ? undefined : JSON.stringify(body) });
  const type = res.headers.get('Content-Type') ?? '';
  return { status: res.status, type, headers: res.headers, body: type.includes('json') ? await res.json() : res.status === 204 ? null : new Uint8Array(await res.arrayBuffer()) };
};

describe('fixture sign in', () => {
  it('tells the page what it offers', async () => {
    const got = await api('GET', '/auth/options');
    expect(got.body).toMatchObject({ password: true, passkey: false, oidc: null, server: 'Bottich', minPassword: 12 });
  });

  it('asks jdp for a code, takes 123456 once and the recovery code once', async () => {
    expect((await api('POST', '/auth/login', { username: 'jdp', password: 'falsch' })).status).toBe(401);
    const step = (await api('POST', '/auth/login', { username: 'jdp', password: 'cannoncommander', remember: true })).body;
    expect(step).toMatchObject({ next: 'totp' });
    const done = await api('POST', '/auth/totp', { challenge: step.challenge, code: '123456' });
    expect(done.body).toMatchObject({ next: 'done', me: { name: 'jdp', twofa: true } });
    expect(done.headers.get('Set-Cookie')).toContain('cannoncommander_session');
    expect((await api('POST', '/auth/totp', { challenge: step.challenge, code: '123456' })).status).toBe(410);
    const again = (await api('POST', '/auth/login', { username: 'jdp', password: 'cannoncommander' })).body;
    expect((await api('POST', '/auth/totp', { challenge: again.challenge, code: '123456' })).status).toBe(400);
    expect((await api('POST', '/auth/totp', { challenge: again.challenge, code: 'k7qm-3xrp' })).body.next).toBe('done');
  });

  it('lets neu pick a password before going in', async () => {
    const step = (await api('POST', '/auth/login', { username: 'neu', password: 'cannoncommander' })).body;
    expect(step.next).toBe('password');
    expect((await api('POST', '/auth/password', { challenge: step.challenge, password: 'kurz' })).status).toBe(400);
    expect((await api('POST', '/auth/password', { challenge: step.challenge, password: 'ein-eigenes-passwort' })).body.next).toBe('done');
  });

  it('answers 401 for everything else once signed out', async () => {
    expect((await api('POST', '/auth/logout')).status).toBe(204);
    expect((await api('GET', '/me')).status).toBe(401);
    await api('POST', '/auth/login', { username: 'anna', password: 'cannoncommander' });
    expect((await api('GET', '/me')).status).toBe(200);
  });
});

describe('fixture share links', () => {
  it('keeps the folder behind its password', async () => {
    expect((await api('GET', '/s/k3x9-urlaub')).body).toEqual({ slug: 'k3x9-urlaub', locked: true });
    expect((await api('GET', '/s/k3x9-urlaub/list')).status).toBe(401);
    expect((await api('POST', '/s/k3x9-urlaub/unlock', { password: 'regen' })).status).toBe(403);
    const { token } = (await api('POST', '/s/k3x9-urlaub/unlock', { password: 'sonnenschein' })).body;
    const open = (await api('GET', '/s/k3x9-urlaub', undefined, { 'X-Share-Token': token })).body;
    expect(open).toMatchObject({ locked: false, kind: 'folder', name: 'Urlaub 2026', uploads: true });
    expect(open.message).toContain('Urlaub');
    const list = (await api('GET', '/s/k3x9-urlaub/list?path=/', undefined, { 'X-Share-Token': token })).body;
    expect(list.items.map((k: { name: string }) => k.name)).toEqual(['Abends', 'Berge.jpg', 'Reiseplan.md', 'Strand.jpg']);
    const zip = await api('GET', '/s/k3x9-urlaub/raw?path=/&download=1', undefined, { 'X-Share-Token': token });
    expect(String.fromCharCode(...(zip.body as Uint8Array).slice(0, 2))).toBe('PK');
  });

  it('takes an upload in chunks and numbers a name that is taken', async () => {
    const { token } = (await api('POST', '/s/k3x9-urlaub/unlock', { password: 'sonnenschein' })).body;
    const h = { 'X-Share-Token': token };
    const up = (await api('POST', '/s/k3x9-urlaub/upload', { path: '/', name: 'Berge.jpg', size: 300 * 1024 }, h)).body;
    expect(up).toMatchObject({ state: 'open', offset: 0 });
    let state = up;
    while (state.state === 'open') {
      const chunk = new Blob([new Uint8Array(Math.min(state.chunk, state.size - state.offset))]);
      state = (await api('PUT', `/s/k3x9-urlaub/upload/${up.id}`, chunk, { ...h, 'Content-Type': 'application/octet-stream', 'Upload-Offset': String(state.offset) })).body;
    }
    expect(state.state).toBe('done');
    const names = (await api('GET', '/s/k3x9-urlaub/list?path=/', undefined, h)).body.items.map((k: { name: string }) => k.name);
    expect(names).toContain('Berge (2).jpg');
  });

  it('says why a link leads nowhere', async () => {
    expect((await api('GET', '/s/abgelaufen')).body).toMatchObject({ status: 410, type: 'share-expired' });
    expect((await api('GET', '/s/aufgebraucht')).body).toMatchObject({ status: 410, type: 'share-used-up' });
    expect((await api('GET', '/s/gibtsnicht')).status).toBe(404);
  });

  it('serves the file of a file link', async () => {
    const open = (await api('GET', '/s/m7tq2p-rechnung')).body;
    expect(open).toMatchObject({ kind: 'file', name: 'Rechnung-Oktober.pdf' });
    const raw = await api('GET', '/s/m7tq2p-rechnung/raw?path=/');
    expect(raw.type).toBe('application/pdf');
    expect(new TextDecoder().decode((raw.body as Uint8Array).slice(0, 8))).toBe('%PDF-1.4');
  });
});
