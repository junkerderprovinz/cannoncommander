import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { toItem } from '@/api/client';
import { prefs } from '@/core/prefs';
import { t } from '@/i18n';
import { shell } from '@/shell/state';
import { createFixture, loadPrototype } from '../../scripts/fixture.js';
import { COL, COLUMN_GROUPS, COLUMNS, Columns, columnLabel, DEFAULT_COLUMNS } from './columns';
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
    expect({ width: mine.width, min: mine.min, r: !!mine.r, mono: !!mine.mono, fixed: !!mine.fixed, sort: mine.sort, sortable: !!mine.num })
      .toEqual({ width: c.width, min: c.min, r: !!c.r, mono: !!c.mono, fixed: !!c.fixed, sort: c.sort, sortable: !!c.num });
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

describe('list width', () => {
  // At 1600 px with the sidebar, two panels and the preview pane, a panel's list is 424 px wide, and on a laptop
  // that shows 1536 px it is 392 px.
  it('fits the default columns beside the sidebar and the preview pane, eliding the name', () => {
    shell.width = 1536;
    Columns.reset();
    expect(parseFloat(Columns.minWidth()) * 16).toBeLessThanOrEqual(392);
  });
});

describe('a name with a width of its own', () => {
  beforeEach(() => {
    shell.width = 1536;
    Columns.reset();
  });
  afterEach(() => {
    shell.width = 1536;
    Columns.reset();
  });

  it('fills the row until somebody drags it', () => {
    expect(Columns.template()).toBe('minmax(0, 1fr) 6.5rem 7.5rem');
  });

  it('keeps its width and lets the last column take the rest of the row', () => {
    prefs.colWidths = { name: 20 };
    expect(Columns.template()).toBe('20rem 6.5rem minmax(7.5rem, 1fr)');
    prefs.colWidths = { name: 20, modified: 9 };
    expect(Columns.template()).toBe('20rem 6.5rem minmax(9rem, 1fr)');
  });

  it('fills the row from its own width up when it stands alone', () => {
    prefs.columns = ['name'];
    prefs.colWidths = { name: 20 };
    expect(Columns.template()).toBe('minmax(20rem, 1fr)');
  });

  it('counts in full in the list width, so a wide name scrolls the list sideways', () => {
    prefs.colWidths = { name: 20 };
    expect(Columns.minWidth()).toBe('37rem');
  });

  it('fills a phone again, beside the size alone', () => {
    shell.width = 500;
    prefs.colWidths = { name: 30 };
    expect(Columns.template()).toBe('minmax(0, 1fr) 6.5rem');
  });
});

describe('size column', () => {
  const folder = (extra: object) => toEntry(toItem({ name: 'filme', dir: true, size: 0, mtime: '2026-11-02T09:00:00.000Z', mode: '0755', ...extra }));

  it('leaves the size of a folder not measured yet blank, and says 0 B for an empty one', () => {
    expect(COL.size!.value(folder({ sizeUnknown: true }), '/mnt/user')).toBe('');
    expect(COL.size!.value(folder({}), '/mnt/user')).toBe('0 B');
  });
});

describe('dragging the line between two columns', () => {
  const start = { size: 6.5, modified: 7.5, type: 8.5 };

  it('moves just that line: one column gains what the other gives up', () => {
    expect(Columns.lineMoved('size', 'modified', start, 1.25)).toMatchObject({ size: 7.75, modified: 6.25 });
    expect(Columns.lineMoved('size', 'modified', start, -2)).toMatchObject({ size: 4.5, modified: 9.5 });
  });

  it('widens and narrows the name on its own, leaving the column beside it as it was', () => {
    const named = { ...start, name: 20 };
    expect(Columns.lineMoved('name', 'size', named, 15)).toEqual({ name: 35 });
    expect(Columns.lineMoved('name', 'size', named, -8)).toEqual({ name: 12 });
  });

  it('stops where a column would get narrower than a short label', () => {
    expect(Columns.lineMoved('size', 'modified', start, 9)).toMatchObject({ size: 11.5, modified: 2.5 });
    expect(Columns.lineMoved('size', 'modified', start, -9)).toMatchObject({ size: 2.5, modified: 11.5 });
    expect(Columns.lineMoved('name', 'size', { ...start, name: 20 }, -30)).toEqual({ name: 7 });
  });

  it('keeps the widths in quarter rems', () => {
    expect(Columns.lineMoved('size', 'modified', start, 0.3)).toMatchObject({ size: 6.75, modified: 7.25 });
  });
});

describe('moving a column by its header', () => {
  afterEach(() => {
    prefs.columns = [...DEFAULT_COLUMNS];
    prefs.colWidths = {};
  });

  it('puts a column where the one it is dropped on stood', () => {
    prefs.columns = ['name', 'size', 'modified', 'type'];
    expect(Columns.moved('modified', 'size')).toEqual(['name', 'modified', 'size', 'type']);
    expect(Columns.moved('size', 'type')).toEqual(['name', 'modified', 'type', 'size']);
  });

  it('keeps the name first, as file managers do', () => {
    prefs.columns = ['name', 'size', 'modified'];
    expect(Columns.moved('modified', 'name')).toEqual(['name', 'size', 'modified']);
    expect(Columns.moved('name', 'modified')).toEqual(['name', 'size', 'modified']);
    prefs.columns = ['size', 'name', 'modified'];
    expect(Columns.shown()).toEqual(['name', 'size', 'modified']);
  });

  it('gives back the default order and widths', () => {
    prefs.columns = ['name', 'modified', 'size'];
    prefs.colWidths = { size: 9, modified: 4 };
    Columns.reset();
    expect(Columns.shown()).toEqual(DEFAULT_COLUMNS);
    expect(prefs.colWidths).toEqual({});
  });
});
