import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';
import { beforeAll, describe, expect, it } from 'vitest';
import { addMessages, setLanguage } from '@/i18n';
import de from './de.json';
import en from './en.json';
import { hexRows, kindOfBytes } from './hex';
import { editable, filmOf, filmTitle, fmtDur, hash, langOf, plural, rng, viewKind } from './kinds';
import { highlight, md } from './markdown';
import { cueAt, parseCues, subsFor } from './renderers/subs';

const repo = join(import.meta.dirname, '..', '..', '..', '..');
const exported = JSON.parse(readFileSync(join(repo, 'testing', 'fixture', 'prototype.json'), 'utf8'));

// The prototype's kit of shared viewer helpers, run in a sandbox so its pure functions can be held against ours.
function prototypeKit() {
  const g = (name: string) => `[${name}]`;
  const esc = (s: unknown) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!);
  const ctx = vm.createContext({ CC: {}, esc, g, FS: { ext: (n: string) => (n.includes('.') ? n.split('.').pop()!.toLowerCase() : '') } });
  vm.runInContext(readFileSync(join(repo, 'prototype', 'src', 'js', 'features', 'c-00-kit.js'), 'utf8'), ctx);
  return (ctx.CC as { ck: { md: (s: string) => string; highlight: (t: string, k: string, e: string) => string; hash: (s: string) => number; rng: (s: string) => () => number; fmtDur: (s: number) => string; filmTitle: (f: string) => { name: string; year: string } } }).ck;
}

interface Node { name: string; dir?: boolean; text?: string; view?: string; children?: Node[]; zip?: Node[] }
const files: Node[] = [];
(function walk(n: Node) {
  for (const k of n.children || []) k.dir ? walk(k) : files.push(k);
})(exported.tree);

beforeAll(() => {
  addMessages('viewer', { de, en });
  setLanguage('de');
});

describe('which renderer a file gets', () => {
  it('agrees with the prototype for every file of the test tree', () => {
    expect(files.filter((n) => n.view && viewKind(n) !== n.view).map((n) => n.name)).toEqual([]);
  });

  it('gives e-books and models a view of their own', () => {
    expect(viewKind({ name: 'Der Name des Windes.epub' })).toBe('epub');
    expect(viewKind({ name: 'Halterung.stl' })).toBe('model');
    expect(viewKind({ name: 'teil.7z.001' })).toBe('file');
  });

  it('opens text, code and markdown for editing and nothing else', () => {
    expect(editable({ name: 'notizen.txt' })).toBe(true);
    expect(editable({ name: 'server.go' })).toBe(true);
    expect(editable({ name: 'setup-notes.md' })).toBe(true);
    expect(editable({ name: 'go' })).toBe(true);
    expect(editable({ name: 'IMG_0203.jpg' })).toBe(false);
    expect(editable({ name: 'docs', dir: true })).toBe(false);
  });

  it('reads rotated logs as text', () => {
    expect(viewKind({ name: 'syslog.1' })).toBe('text');
    expect(viewKind({ name: 'engine.log.2' })).toBe('text');
  });

  it('names the language of a file', () => {
    expect(langOf({ name: 'docker-compose.yml' })).toBe('YAML');
    expect(langOf({ name: '44.conf' })).toBe('Nginx');
    expect(langOf({ name: 'liste.txt' })).toBe('Text');
  });
});

describe('the helpers the prototype shares', () => {
  const kit = prototypeKit();

  it('renders every markdown file of the test tree as the prototype does', () => {
    const notes = files.filter((n) => n.name.endsWith('.md') && n.text);
    expect(notes.length).toBeGreaterThan(2);
    // The prototype draws the tick with its glyph helper, the app with the glyph's svg; the rest is the same markup.
    const same = (html: string) => html.replace(/<svg[^]*?<\/svg>|\[IconCheck\]/g, '✓');
    for (const n of notes) expect(same(md(n.text!))).toBe(same(kit.md(n.text!)));
  });

  it('highlights every line of code as the prototype does', () => {
    for (const n of files.filter((f) => f.text && ['code', 'text'].includes(f.view!))) {
      const e = n.name.includes('.') ? n.name.split('.').pop()!.toLowerCase() : '';
      for (const line of n.text!.split('\n').slice(0, 80)) expect(highlight(line, n.view!, e)).toBe(kit.highlight(line, n.view!, e));
    }
  });

  it('keeps links to the web and drops script addresses', () => {
    expect(md('[Handbuch](https://docs.unraid.net)')).toContain('href="https://docs.unraid.net"');
    expect(md('[klick](javascript:void)')).toBe('<p>klick</p>');
    expect(md('<img src=x onerror=alert(1)>')).toBe('<p>&lt;img src=x onerror=alert(1)&gt;</p>');
  });

  it('draws the same made-up numbers from the same seed', () => {
    for (const s of ['IMG_0203.jpg', '/mnt/user/musik', '']) {
      expect(hash(s)).toBe(kit.hash(s));
      const a = rng(s), b = kit.rng(s);
      for (let i = 0; i < 5; i++) expect(a()).toBe(b());
    }
  });

  it('writes durations and film titles alike', () => {
    for (const s of [0, 59, 61, 3599, 3600, 5025.4]) expect(fmtDur(s)).toBe(kit.fmtDur(s));
    for (const f of ['Big Buck Bunny (2008)', 'Big.Buck.Bunny.2008.1080p.mkv', 'Sintel']) expect(filmTitle(f)).toEqual({ ...kit.filmTitle(f) });
  });

  it('names a film after its folder when the folder carries the year', () => {
    expect(filmOf('film.mkv', '/mnt/user/filme/Sintel (2010)')).toBe('Sintel (2010)');
    expect(filmOf('Big.Buck.Bunny.2008.1080p.mkv', '/mnt/user/downloads')).toBe('Big.Buck.Bunny.2008.1080p.mkv');
  });

  it('counts in both languages', () => {
    expect(plural(1, 'lines')).toBe('1 Zeile');
    expect(plural(1675, 'lines')).toBe(`${(1675).toLocaleString('de-AT')} Zeilen`);
    setLanguage('en');
    expect(plural(2, 'pages')).toBe('2 pages');
    setLanguage('de');
  });
});

describe('subtitles', () => {
  const srt = '1\n00:00:04,200 --> 00:00:07,400\nHörst du das?\n\n2\n00:00:08,200 --> 00:00:11,400\n<i>Da ist jemand</i>\nim Wald.\n';

  it('reads SRT and WebVTT cues', () => {
    expect(parseCues(srt)).toEqual([{ start: 4.2, end: 7.4, text: 'Hörst du das?' }, { start: 8.2, end: 11.4, text: 'Da ist jemand\nim Wald.' }]);
    expect(parseCues('WEBVTT\n\n00:01.000 --> 00:02.500 align:start\nHallo\n')).toEqual([{ start: 1, end: 2.5, text: 'Hallo' }]);
    expect(cueAt(parseCues(srt), 9)).toBe('Da ist jemand\nim Wald.');
    expect(cueAt(parseCues(srt), 7.9)).toBe('');
  });

  it('finds the files beside a film and their language', () => {
    const names = ['Big Buck Bunny (2008).mkv', 'Big Buck Bunny (2008).de.srt', 'Big Buck Bunny (2008).en.forced.vtt', 'Big Buck Bunny (2008).srt', 'Sintel.srt', 'poster.jpg'];
    expect(subsFor('Big Buck Bunny (2008).mkv', names)).toEqual([
      { name: 'Big Buck Bunny (2008).de.srt', label: 'Deutsch' },
      { name: 'Big Buck Bunny (2008).en.forced.vtt', label: 'Englisch' },
      { name: 'Big Buck Bunny (2008).srt', label: 'Untertitel-Datei' },
    ]);
  });
});

describe('the hex view', () => {
  it('lays out sixteen bytes a row with their printable characters', () => {
    const rows = hexRows(new TextEncoder().encode('SQLite format 3\0' + 'ab'));
    expect(rows).toEqual([
      { o: '00000000', h: '53 51 4c 69 74 65 20 66 6f 72 6d 61 74 20 33 00', a: 'SQLite format 3.' },
      { o: '00000010', h: '61 62', a: 'ab' },
    ]);
  });

  it('knows a database and a program by their first bytes', () => {
    expect(kindOfBytes(new TextEncoder().encode('SQLite format 3\0'))).toBe('sqlite');
    expect(kindOfBytes(Uint8Array.of(0x7f, 0x45, 0x4c, 0x46, 2))).toBe('program');
    expect(kindOfBytes(Uint8Array.of(1, 2, 3))).toBe('binary');
  });
});
