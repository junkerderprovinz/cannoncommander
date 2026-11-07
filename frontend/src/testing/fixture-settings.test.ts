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
  const post = async (path: string, body: unknown) => (await fetch(base + path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })).json();
  return { post, close: () => server.close() };
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
