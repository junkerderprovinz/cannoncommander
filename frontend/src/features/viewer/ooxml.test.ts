import { describe, expect, it } from 'vitest';
import { office } from '../../../../testing/generate/formats.mjs';
import { cellRef, colName, readCsv, readPptx, readXlsx } from './ooxml';

// The test tree's generator writes real Office files; these readers have to take them as Office would.
const bytes = (layout: { length: number; chunks: { offset: number; buf: Uint8Array }[] }) => {
  const out = new Uint8Array(layout.length);
  for (const c of layout.chunks) out.set(c.buf, c.offset);
  return out;
};
const rows = [['Posten', 'Betrag'], ['Miete', 9600], ['Strom', 840], ['Summe', 10440]];
const T = Date.UTC(2026, 9, 1);
const lines = ['Datei Präsentation.pptx', 'Erstellt für die Tests von CannonCommander.'];

describe('spreadsheets', () => {
  it('reads cells, numbers and the sheet name of an xlsx file', () => {
    const [sheet] = readXlsx(bytes(office('xlsx', { title: 'Steuer 2025', lines: [], rows, time: T, size: 0 })));
    expect(sheet!.name).toBe('Steuer 2025');
    expect(sheet!.rows.map((r) => r.map((c) => c.v))).toEqual([['Posten', 'Betrag'], ['Miete', '9600'], ['Strom', '840'], ['Summe', '10440']]);
    expect(sheet!.rows[1]![1]!.num).toBe(true);
  });

  it('turns cell names into places and back', () => {
    expect(cellRef('A1')).toEqual([0, 0]);
    expect(cellRef('AB12')).toEqual([11, 27]);
    expect([0, 25, 26, 27, 701].map(colName)).toEqual(['A', 'Z', 'AA', 'AB', 'ZZ']);
  });

  it('reads csv with quotes, separators inside them and semicolons', () => {
    expect(readCsv('a,"b, c",d\n1,"say ""hi""",3\n').map((r) => r.map((c) => c.v))).toEqual([['a', 'b, c', 'd'], ['1', 'say "hi"', '3']]);
    expect(readCsv('Monat;Betrag\r\nJänner;3.100,00\r\n').map((r) => r.map((c) => c.v))).toEqual([['Monat', 'Betrag'], ['Jänner', '3.100,00']]);
    expect(readCsv('x\ty\n1\t2', '\t')[1]!.map((c) => c.num)).toEqual([true, true]);
  });
});

describe('presentations', () => {
  it('reads the title and the lines of every slide', () => {
    const got = readPptx(bytes(office('pptx', { title: 'Präsentation', lines, time: T, size: 0 })));
    expect(got.slides).toEqual([{ title: 'Präsentation', lines }]);
    expect(got.thumbnail).toBeUndefined();
  });

  it('hands over the preview picture Office keeps in the file', () => {
    const picture = Uint8Array.of(0xff, 0xd8, 0xff, 0xd9);
    const got = readPptx(bytes(office('pptx', { title: 'Mit Bild', lines, thumbnail: picture, time: T, size: 0 })));
    expect([...got.thumbnail!]).toEqual([...picture]);
  });
});
