// @vitest-environment node
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { describe, expect, it } from 'vitest';
import { settingsFixtureMiddleware } from '../../scripts/fixture-settings.js';

// A fresh fixture server, as the parity runner starts one for every step file.
async function fresh() {
  const settings = settingsFixtureMiddleware();
  const server = createServer((req, res) => settings(req as never, res as never, () => { res.statusCode = 404; res.end(); }));
  await new Promise<void>((r) => server.listen(0, '127.0.0.1', () => r()));
  const base = `http://127.0.0.1:${(server.address() as AddressInfo).port}/api/v1`;
  const send = (method: string) => async (path: string, body: unknown) => {
    const res = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    return res.status === 204 ? null : res.json();
  };
  const get = async (path: string) => (await fetch(base + path)).json();
  return { post: send('POST'), patch: send('PATCH'), get, close: () => server.close() };
}

describe('the secrets of the fixture', () => {
  it('come out the same from every fresh server, so their pictures match from run to run', async () => {
    const take = async () => {
      const fx = await fresh();
      const totp = await fx.post('/me/totp', {});
      const codes = await fx.post('/me/totp', { regenerate: true });
      fx.close();
      return { secret: totp.secret, qr: totp.qr, codes: codes.codes };
    };
    const one = await take(), two = await take();
    expect(one.secret).toMatch(/^[A-Z2-7]{32}$/);
    expect(two).toEqual(one);
  });

  it('still differ from one request to the next', async () => {
    const fx = await fresh();
    const a = await fx.post('/me/totp', { regenerate: true }), b = await fx.post('/me/totp', { regenerate: true });
    fx.close();
    expect(new Set(a.codes).size).toBe(8);
    expect(b.codes).not.toEqual(a.codes);
  });
});

describe('the MCP keys of the fixture', () => {
  it("list the account's own keys, and everybody's for an admin", async () => {
    const fx = await fresh();
    const mine = await fx.get('/mcp/keys'), all = await fx.get('/mcp/keys?all=true');
    fx.close();
    expect(mine.keys.map((k: { owner: string }) => k.owner)).toEqual(['jdp', 'jdp']);
    expect(all.keys.some((k: { owner: string; own: boolean }) => k.owner === 'anna' && !k.own)).toBe(true);
  });

  it('make a key only with the password, reading unless asked otherwise', async () => {
    const fx = await fresh();
    const refused = await fx.post('/mcp/keys', { label: 'Zed', client: 'zed' });
    const made = await fx.post('/mcp/keys', { label: 'Zed', client: 'zed', password: 'geheim' });
    fx.close();
    expect(refused.status).toBe(400);
    expect(made.key).toMatch(/^ccmcp_/);
    expect(made.item).toMatchObject({ label: 'Zed', canWrite: false, own: true });
  });

  it('let a key write only with the password', async () => {
    const fx = await fresh();
    const { keys } = await fx.get('/mcp/keys');
    const reader = keys.find((k: { canWrite: boolean }) => !k.canWrite);
    const refused = await fx.patch(`/mcp/keys/${reader.id}`, { canWrite: true });
    const allowed = await fx.patch(`/mcp/keys/${reader.id}`, { canWrite: true, password: 'geheim' });
    fx.close();
    expect(refused.status).toBe(400);
    expect(allowed.canWrite).toBe(true);
  });
});
