import { describe, expect, it } from 'vitest';
import { toItem } from '@/api/client';
import { t } from '@/i18n';
import { createFixture, loadPrototype } from '../../scripts/fixture.js';
import { cut, prototypeSource, runPrototype } from '../testing/prototype';
import { Columns } from './columns';
import { type Entry, GROUPS, groupEntries, groupHeads, nameMatcher, shown, sortEntries, toEntry } from './entries';
import { viewOf } from './views';

const files = prototypeSource('30-files.js');
const P = loadPrototype();
const fx = createFixture();

// The prototype's sort and grouping, run on its own tree in its own sandbox.
P.run(cut(files, 'const AGE =', '\n};\n'));
P.run("var groupOf = (t) => CC.prefs.folderGroups?.[t.path] || '';");
type Tab = { path: string; sort: string; dir: number };
const protoOrder = P.run<(list: unknown[], t: Tab) => { name: string }[]>(
  `(list, t) => { ${cut(files, '    const by = {', 'if (grp) list.sort((a, b) => grp.rank(a) - grp.rank(b));')}; return list; }`,
);
const protoViewOf = P.run<(t: { path: string }) => string>(`(${cut(files, 'const viewOf = (t) => {', '\n}').replace('const viewOf = ', '')})`);

const FOLDERS = ['/mnt/user/downloads', '/mnt/user/dokumente', '/mnt/user/fotos/2026', '/mnt/user/backups', '/mnt/user', '/mnt/user/appdata', '/mnt'];
const META = ['duration', 'pages', 'dimensions'];

function appList(path: string): Entry[] {
  const items = fx.list(path)!.items.map(toItem).filter(shown(true)).map(toEntry);
  const meta = fx.meta({ paths: items.map((k) => `${path}/${k.name}`), columns: META }).items;
  for (const k of items) k.meta = meta[`${path}/${k.name}`] ?? {};
  return items;
}
const protoList = (path: string) => P.FS.kids(P.FS.nodeAt(path)).map((k) => ({ ...k, kind: P.FS.kindOf(k) }));

describe('nameMatcher', () => {
  const proto = runPrototype<(q: string) => (k: { name: string }) => boolean>(
    `${cut(files, 'function nameMatcher(query) {', '\n}\n')}; nameMatcher`, { Labels: { matches: () => false } },
  );
  const names = ['Big.Buck.Bunny.2008.1080p.mkv', 'album.flac', 'IMG_0201.jpg', 'setup-notes.md', 'Steuer 2025.xlsx', 'a+b (1).txt', '.DS_Store'];
  const queries = ['mkv', '*.mkv', 'IMG_02?1.jpg', 'notes flac', 'STEUER', '*.(1).txt', 'a+b', '?', '*', '', '  bunny   ', '#rot'];
  it.each(queries)('matches the same names as the prototype for %j', (q) => {
    expect(names.filter((n) => nameMatcher(q)({ name: n } as never))).toEqual(names.filter((n) => proto(q)({ name: n })));
  });
  it('asks the tag matchers for a word with #', () => {
    const match = nameMatcher('#urlaub', (k, w) => k.name === 'IMG_0201.jpg' && w === '#urlaub');
    expect(names.filter((n) => match({ name: n } as never))).toEqual(['IMG_0201.jpg']);
  });
});

describe('entries chain', () => {
  const SORTS = ['name', 'size', 'time', 'type', 'created', 'items', 'ext', 'owner', 'inode', 'hardlinks', 'duration', 'pages', 'dimensions'];
  const cases = FOLDERS.flatMap((path) => SORTS.flatMap((sort) => [1, -1].map((dir) => [path, sort, dir] as const)));

  it.each(cases)('orders %s by %s (%i) as the prototype does', (path, sort, dir) => {
    for (const foldersFirst of [true, false]) {
      P.CC.prefs.foldersFirst = foldersFirst;
      P.CC.prefs.folderGroups = {};
      const want = protoOrder(protoList(path), { path, sort, dir }).map((k) => k.name);
      const got = sortEntries(appList(path), { sort, dir, foldersFirst, comparer: Columns.comparer(sort) }).map((k) => k.name);
      expect(got).toEqual(want);
    }
  });

  it.each(FOLDERS.flatMap((path) => Object.keys(GROUPS).map((g) => [path, g] as const)))('groups %s by %s as the prototype does', (path, group) => {
    P.CC.prefs.foldersFirst = true;
    P.CC.prefs.folderGroups = { [path]: group };
    const want = protoOrder(protoList(path), { path, sort: 'name', dir: 1 }).map((k) => k.name);
    const list = groupEntries(sortEntries(appList(path), { sort: 'name', dir: 1, foldersFirst: true }), group, P.NOW);
    expect(list.map((k) => k.name)).toEqual(want);
  });

  it('names each section as the prototype does and counts its rows', () => {
    const protoName = P.run<(g: string, k: unknown) => string>('(g, k) => GROUPS[g].name(k)');
    for (const path of FOLDERS) {
      for (const group of Object.keys(GROUPS)) {
        const list = groupEntries(sortEntries(appList(path), { sort: 'name', dir: 1, foldersFirst: true }), group, P.NOW);
        const heads = groupHeads(list, group, P.NOW);
        list.forEach((k, i) => {
          const proto = protoList(path).find((x) => x.name === k.name)!;
          expect(t(`groups.${GROUPS[group]!.name(k, P.NOW)}`)).toBe(protoName(group, proto));
          if (heads[i]) expect(heads[i]!.count).toBe(list.filter((x) => GROUPS[group]!.name(x, P.NOW) === heads[i]!.name).length);
        });
        expect(heads.filter(Boolean).length).toBe(new Set(list.map((k) => GROUPS[group]!.name(k, P.NOW))).size);
      }
    }
  });
});

describe('viewOf', () => {
  const holds = (path: string) => !!fx.tree(path)?.items.some((k) => k.total);
  const settings: Record<string, string>[] = [{}, { '/mnt/user/filme': 'columns' }, { '/mnt/user': 'columns', '/mnt/user/fotos': 'gallery' }, { '/mnt': 'tiles' }];
  const paths = ['/', '/mnt', '/mnt/user', '/mnt/user/filme', '/mnt/user/filme/Sintel (2010)', '/mnt/user/fotos/2026', '/boot/config'];
  it.each(settings)('picks the view the prototype picks with %j', (folderViews) => {
    P.CC.prefs.folderViews = folderViews;
    for (const path of paths) expect(viewOf(path, folderViews, holds(path)), path).toBe(protoViewOf({ path }));
  });
});
