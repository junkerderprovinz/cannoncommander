import { describe, expect, it } from 'vitest';
import { toItem } from '@/api/client';
import { t } from '@/i18n';
import { createFixture, loadPrototype } from '../../scripts/fixture.js';
import { COL, COLUMN_GROUPS, COLUMNS, columnLabel } from './columns';
import { toEntry } from './entries';
import { join } from './paths';

const P = loadPrototype();
const fx = createFixture();
P.CC.prefs.folderSizes = true;
const protoColumns = Object.values(P.COL);
const GROUP_NAMES = P.run<string[]>('COLUMN_GROUPS');

// Every folder of the made-up tree with its entries, each with the values fs/meta gives for it.
const rows: { path: string; name: string }[] = [];
(function walk(path: string) {
  for (const k of fx.list(path)?.items ?? []) {
    rows.push({ path, name: k.name });
    if (k.dir) walk(join(path, k.name));
  }
})('/');
const metaIds = COLUMNS.filter((c) => c.meta).map((c) => c.id);
const meta = fx.meta({ paths: rows.map((r) => join(r.path, r.name)), columns: metaIds }).items;
const appEntry = (path: string, name: string) => ({ ...toEntry(toItem(fx.list(path)!.items.find((k) => k.name === name)!)), meta: meta[join(path, name)] });
const protoNode = (path: string, name: string) => {
  const k = P.FS.nodeAt(join(path, name))!;
  return { ...k, kind: P.FS.kindOf(k) };
};

describe('columns', () => {
  it('has every column of the prototype, in its group and order', () => {
    expect(COLUMNS.length).toBe(protoColumns.length);
    const order = (cols: { id: string; group: string }[], group: string) => cols.filter((c) => c.group === group).map((c) => c.id);
    COLUMN_GROUPS.forEach(([id], i) => expect(order(COLUMNS, id)).toEqual(order(protoColumns, GROUP_NAMES[i]!)));
  });

  it.each(protoColumns.map((c) => [c.id, c] as const))('lays out %s like the prototype', (id, c) => {
    const mine = COL[id]!;
    expect(columnLabel(id)).toBe(c.label);
    expect({ width: mine.width, r: !!mine.r, mono: !!mine.mono, fixed: !!mine.fixed, sort: mine.sort, sortable: !!mine.num })
      .toEqual({ width: c.width, r: !!c.r, mono: !!c.mono, fixed: !!c.fixed, sort: c.sort, sortable: !!c.num });
    expect(t(`columnGroups.${mine.group}`)).toBe(c.group);
  });

  // The exported tree carries what the prototype showed after all its features; the column functions fill in the rest.
  // The prototype makes up an archive bit for a fifth of the files; Linux has none, so the column leaves it out.
  const proto = (id: string, path: string, name: string) => {
    const node = protoNode(path, name), cols = (node as { cols?: Record<string, string> }).cols;
    const v = cols?.[id] ?? P.COL[id]!.value?.(node, path) ?? name;
    return id === 'attributes' ? v.replace('A', '') : v;
  };
  it.each(COLUMNS.map((c) => c.id))('writes every value of %s as the prototype does', (id) => {
    for (const { path, name } of rows) {
      expect(COL[id]!.value(appEntry(path, name), path), `${path}/${name}`).toBe(proto(id, path, name));
    }
  });

  // Where the prototype shows nothing, its figure only parks the row at one end, as the -1 of a missing value does.
  it.each(COLUMNS.filter((c) => c.num).map((c) => c.id))('sorts %s by the same figures as the prototype', (id) => {
    for (const { path, name } of rows.filter((r) => proto(id, r.path, r.name))) {
      expect(COL[id]!.num!(appEntry(path, name)), `${path}/${name}`).toBe(P.COL[id]!.num!(protoNode(path, name)));
    }
  });
});
