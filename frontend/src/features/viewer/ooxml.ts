// Just enough of xlsx, csv and pptx for the built-in office preview: cells with their formulas, and the text of
// each slide with the preview picture Office keeps in the file.
import { strFromU8, unzipSync } from 'fflate';

export interface Cell { v: string; f?: string; num?: boolean }
export interface Sheet { name: string; rows: Cell[][] }
export interface Slide { title: string; lines: string[] }

const xml = (s: string) => new DOMParser().parseFromString(s, 'application/xml');
const all = (n: Document | Element, local: string) => [...n.getElementsByTagNameNS('*', local)];
const one = (n: Document | Element, local: string) => n.getElementsByTagNameNS('*', local)[0];
const text = (n: Element | undefined) => (n ? all(n, 't').map((x) => x.textContent ?? '').join('') : '');

// "C12" is row 11, column 2, counted from zero.
export function cellRef(ref: string): [number, number] {
  const m = ref.match(/^([A-Z]+)(\d+)$/)!;
  const col = [...m[1]!].reduce((a, c) => a * 26 + c.charCodeAt(0) - 64, 0) - 1;
  return [+m[2]! - 1, col];
}
export const colName = (c: number): string => (c < 26 ? String.fromCharCode(65 + c) : colName(Math.floor(c / 26) - 1) + String.fromCharCode(65 + (c % 26)));

const unzip = (data: Uint8Array) => {
  const files = unzipSync(data);
  return (name: string) => (files[name] ? strFromU8(files[name]!) : '');
};

export function readXlsx(data: Uint8Array, fmt: (n: number) => string = String): Sheet[] {
  const part = unzip(data);
  const shared = all(xml(part('xl/sharedStrings.xml') || '<sst/>'), 'si').map((si) => text(si));
  const rels = Object.fromEntries(all(xml(part('xl/_rels/workbook.xml.rels') || '<r/>'), 'Relationship').map((r) => [r.getAttribute('Id'), r.getAttribute('Target')]));
  return all(xml(part('xl/workbook.xml')), 'sheet').map((s, k) => {
    const rid = s.getAttributeNS('http://schemas.openxmlformats.org/officeDocument/2006/relationships', 'id') || s.getAttribute('r:id');
    const target = (rels[rid ?? ''] || `worksheets/sheet${k + 1}.xml`).replace(/^\/?xl\//, '').replace(/^\//, '');
    const rows: Cell[][] = [];
    for (const c of all(xml(part('xl/' + target) || '<w/>'), 'c')) {
      const [r, col] = cellRef(c.getAttribute('r')!);
      const type = c.getAttribute('t'), v = one(c, 'v')?.textContent ?? '', f = one(c, 'f')?.textContent || undefined;
      let cell: Cell;
      if (type === 's') cell = { v: shared[+v] ?? '' };
      else if (type === 'inlineStr') cell = { v: text(one(c, 'is')) };
      else if (type === 'b') cell = { v: v === '1' ? 'WAHR' : 'FALSCH' };
      else if (type === 'str' || type === 'e') cell = { v };
      else cell = v === '' ? { v: '' } : { v: fmt(+v), num: true };
      if (f) cell.f = '=' + f;
      (rows[r] ??= [])[col] = cell;
    }
    return { name: s.getAttribute('name') || String(k + 1), rows };
  });
}

// Quotes may hold the separator and line breaks; two quotes in a row are one.
export function readCsv(src: string, sep = src.includes('\t') && !src.includes(';') && !src.includes(',') ? '\t' : src.split('\n')[0]!.includes(';') ? ';' : ','): Cell[][] {
  const rows: Cell[][] = [[]];
  let cur = '', quoted = false;
  const push = () => { rows.at(-1)!.push({ v: cur, num: /^-?[\d.,\s]+$/.test(cur.trim()) && cur.trim() !== '' }); cur = ''; };
  for (let i = 0; i < src.length; i++) {
    const c = src[i]!;
    if (quoted) {
      if (c === '"' && src[i + 1] === '"') { cur += '"'; i++; } else if (c === '"') quoted = false; else cur += c;
    } else if (c === '"') quoted = true;
    else if (c === sep) push();
    else if (c === '\n') { push(); rows.push([]); } else if (c !== '\r') cur += c;
  }
  if (cur || rows.at(-1)!.length) push();
  if (!rows.at(-1)!.length) rows.pop();
  return rows;
}

export function readPptx(data: Uint8Array): { slides: Slide[]; thumbnail?: Uint8Array } {
  const files = unzipSync(data);
  const slides = Object.keys(files)
    .map((name) => [name, name.match(/^ppt\/slides\/slide(\d+)\.xml$/)?.[1]] as const)
    .filter((x): x is readonly [string, string] => !!x[1])
    .sort((a, b) => +a[1] - +b[1])
    .map(([name]) => {
      const paras = all(xml(strFromU8(files[name]!)), 'p').filter((p) => p.localName === 'p' && p.namespaceURI?.includes('drawingml'))
        .map((p) => text(p).trim()).filter(Boolean);
      return { title: paras[0] ?? '', lines: paras.slice(1) };
    });
  return { slides, thumbnail: files['docProps/thumbnail.jpeg'] };
}
