// @vitest-environment node
// The fixture's batch rename against the prototype's own preview, on the same tree: every token, the counter, find
// and replace, the letter case and the row errors have to come out the same.
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { opsFixture } from '../../scripts/fixture-ops.js';
import { toolsFixture } from '../../scripts/fixture-tools.js';
import { loadPrototype } from '../../scripts/fixture.js';
import { cut, prototypeSource } from './prototype';

const P = loadPrototype();
const source = prototypeSource('features/b-60-rename.js');
type Row = { node: { name: string }; old: string; out: string; err: string; missing: string[] };
const preview = P.run<(nodes: unknown[], o: Record<string, unknown>, dir: string) => { rows: Row[]; bad: boolean }>(
  `(() => { const bk = { kidsOf: (d) => FS.kids(FS.nodeAt(d)) }; ${cut(source, '  const isoDate =', '    return { rows, bad };\n  }')}; return preview; })()`,
);
const LABEL: Record<string, string> = { A: 'Aufnahmedatum', K: 'Kamera', I: 'Interpret', T: 'Titel', R: 'Auflösung', S: 'Staffel', F: 'Folge' };
const ERROR: Record<string, string> = { empty: 'leer', invalid: 'unzulässig', exists: 'gibt es schon', duplicate: 'doppelt' };

let server: Server, base = '';
beforeAll(async () => {
  const tools = opsFixture(undefined, [toolsFixture]);
  server = createServer((req, res) => tools(req as never, res as never, () => { res.statusCode = 404; res.end(); }));
  await new Promise<void>((r) => server.listen(0, '127.0.0.1', () => r()));
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}/api/v1`;
});
afterAll(() => server.close());

async function dryRun(paths: string[], mask: Record<string, unknown>) {
  const res = await fetch(base + '/fs/rename/batch', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ paths, mask, dryRun: true }) });
  return { status: res.status, body: await res.json() };
}

const mask = (name: string, more: Record<string, unknown> = {}) => ({ name, ext: '[E]', start: 1, step: 1, find: '', repl: '', regex: false, case: 'keep', ...more });
const CASES: [string, string, Record<string, unknown>][] = [
  ['/mnt/user/fotos/Urlaub', '[N]', mask('[N]')],
  ['/mnt/user/fotos/Urlaub', 'a counter with three digits', mask('Urlaub_[C3]', { case: 'lower' })],
  ['/mnt/user/fotos/Urlaub', 'a counter from 10 in steps of 5', mask('[N]-[C2]', { start: 10, step: 5 })],
  ['/mnt/user/fotos/Urlaub', 'date taken and camera', mask('[A]_[K]_[C3]')],
  ['/mnt/user/fotos/2026', 'date and folder', mask('[P] [D]')],
  ['/mnt/user/fotos/2026', 'resolution', mask('[N] [R]', { case: 'upper' })],
  ['/mnt/user/musik', 'artist and title, empty where a file has none', mask('[I] - [T]')],
  ['/mnt/user/serien/Pioneer One/Staffel 1', 'season and episode', mask('S[S2]E[F2]')],
  ['/mnt/user/dokumente', 'every word capitalised', mask('[N]', { case: 'title' })],
  ['/mnt/user/dokumente', 'plain find ignores case', mask('[N]', { find: 'RECHNUNG', repl: 'Beleg' })],
  ['/mnt/user/dokumente', 'a regular expression with groups', mask('[N]', { find: '^(\\w)(\\w+)', repl: '$2$1', regex: true })],
  ['/mnt/user/dokumente', 'a new ending', mask('[N]', { ext: 'bak' })],
  ['/mnt/user/dokumente', 'the same name twice', mask('gleich')],
  ['/mnt/user/dokumente', 'an empty name', mask('', { ext: '' })],
  ['/mnt/user/dokumente', 'a slash', mask('a/[N]')],
  ['/mnt/user/downloads', 'a name another entry has', mask('setup-notes', { ext: 'md' })],
];

describe('batch rename masks', () => {
  it.each(CASES)('%s: %s comes out as in the prototype', async (dir, _what, o) => {
    const nodes = P.FS.kids(P.FS.nodeAt(dir)).filter((k) => !(k as { dir?: boolean }).dir).slice(0, 6);
    const want = preview(nodes, o, dir);
    const got = await dryRun(nodes.map((k) => `${dir}/${k.name}`), o);
    expect(got.status).toBe(200);
    expect(got.body.rows.map((r: { old: string; new: string; error?: string; missing?: string[] }) => [r.old, r.new, r.error ? ERROR[r.error] : '', (r.missing ?? []).map((m) => LABEL[m]).sort()]))
      .toEqual(want.rows.map((r) => [r.old, r.out, r.err, [...r.missing].sort()]));
  });

  it('calls a broken expression bad, as the prototype does', async () => {
    const dir = '/mnt/user/dokumente', nodes = P.FS.kids(P.FS.nodeAt(dir)).slice(0, 2);
    expect(preview(nodes, mask('[N]', { find: '(', regex: true }), dir).bad).toBe(true);
    expect((await dryRun(nodes.map((k) => `${dir}/${k.name}`), mask('[N]', { find: '(', regex: true }))).status).toBe(400);
  });
});
