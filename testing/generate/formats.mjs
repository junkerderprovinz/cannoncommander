// Real file formats at the sizes the prototype lists. Every builder returns a layout of byte chunks at absolute
// offsets plus a total length; whatever lies between the chunks stays a hole in a sparse file.
import zlib from 'node:zlib';

const CRC_TABLE = Array.from({ length: 256 }, (_, n) => {
  let c = n;
  for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  return c >>> 0;
});

export function crc32(buf, crc = 0) {
  let c = ~crc >>> 0;
  for (let i = 0; i < buf.length; i++) c = CRC_TABLE[(c ^ buf[i]) & 0xff] ^ (c >>> 8);
  return ~c >>> 0;
}

const ZEROS = Buffer.alloc(1 << 16);
function crc32Zeros(crc, n) {
  for (; n > 0; n -= ZEROS.length) crc = crc32(n >= ZEROS.length ? ZEROS : ZEROS.subarray(0, n), crc);
  return crc;
}

const layout = (length, ...chunks) => ({ length, chunks: chunks.filter((c) => c.buf.length) });

function dosTime(ms) {
  const d = new Date(ms);
  return {
    time: (d.getUTCHours() << 11) | (d.getUTCMinutes() << 5) | (d.getUTCSeconds() >> 1),
    date: ((d.getUTCFullYear() - 1980) << 9) | ((d.getUTCMonth() + 1) << 5) | d.getUTCDate(),
  };
}

// Stored entries only: the content of a test file matters more than its ratio, and stored zeros can stay holes.
// Each entry is { name, data, size, time } where size may exceed data.length and the rest reads as zeros.
// A size larger than the entries leaves a gap before the central directory, which readers skip.
export function zip(entries, { size = 0 } = {}) {
  const chunks = [];
  const central = [];
  let at = 0;
  for (const e of entries) {
    const name = Buffer.from(e.name, 'utf8');
    const data = e.data || Buffer.alloc(0);
    const total = e.dir ? 0 : Math.max(e.size ?? data.length, data.length);
    const crc = e.dir ? 0 : crc32Zeros(crc32(data), total - data.length);
    const { time, date } = dosTime(e.time ?? Date.UTC(2026, 9, 1));
    const head = Buffer.alloc(30);
    head.writeUInt32LE(0x04034b50, 0);
    head.writeUInt16LE(20, 4);
    head.writeUInt16LE(0x0800, 6);
    head.writeUInt16LE(0, 8);
    head.writeUInt16LE(time, 10);
    head.writeUInt16LE(date, 12);
    head.writeUInt32LE(crc, 14);
    head.writeUInt32LE(total, 18);
    head.writeUInt32LE(total, 22);
    head.writeUInt16LE(name.length, 26);
    chunks.push({ offset: at, buf: Buffer.concat([head, name, data]) });
    const cd = Buffer.alloc(46);
    cd.writeUInt32LE(0x02014b50, 0);
    cd.writeUInt16LE(0x031e, 4);
    cd.writeUInt16LE(20, 6);
    cd.writeUInt16LE(0x0800, 8);
    cd.writeUInt16LE(time, 12);
    cd.writeUInt16LE(date, 14);
    cd.writeUInt32LE(crc, 16);
    cd.writeUInt32LE(total, 20);
    cd.writeUInt32LE(total, 24);
    cd.writeUInt16LE(name.length, 28);
    cd.writeUInt32LE(((e.dir ? 0o40755 : 0o100644) << 16) >>> 0, 38);
    cd.writeUInt32LE(at, 42);
    central.push(Buffer.concat([cd, name]));
    at += 30 + name.length + total;
  }
  const dir = Buffer.concat(central);
  const end = Buffer.alloc(22);
  const natural = at + dir.length + end.length;
  if (Math.max(size, natural) > 0xffffffff) throw new Error('zip larger than 4 GiB needs zip64');
  const dirAt = Math.max(at, size - dir.length - end.length);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(entries.length, 8);
  end.writeUInt16LE(entries.length, 10);
  end.writeUInt32LE(dir.length, 12);
  end.writeUInt32LE(dirAt, 16);
  chunks.push({ offset: dirAt, buf: Buffer.concat([dir, end]) });
  return layout(dirAt + dir.length + end.length, ...chunks);
}

function tarHeader(name, size, mtime, mode = 0o644) {
  const h = Buffer.alloc(512);
  const put = (s, off, len) => h.write(s, off, len, 'utf8');
  const oct = (n, len) => n.toString(8).padStart(len - 1, '0') + '\0';
  put(name, 0, 100);
  put(oct(mode, 8), 100, 8);
  put(oct(99, 8), 108, 8);
  put(oct(100, 8), 116, 8);
  put(oct(size, 12), 124, 12);
  put(oct(Math.floor(mtime / 1000), 12), 136, 12);
  put('        ', 148, 8);
  put('0', 156, 1);
  put('ustar\0' + '00', 257, 8);
  put('nobody', 265, 32);
  put('users', 297, 32);
  let sum = 0;
  for (const b of h) sum += b;
  put(sum.toString(8).padStart(6, '0') + '\0 ', 148, 8);
  return h;
}

export function tar(files) {
  const parts = [];
  for (const f of files) {
    parts.push(tarHeader(f.name, f.data.length, f.time), f.data, Buffer.alloc((512 - (f.data.length % 512)) % 512));
  }
  parts.push(Buffer.alloc(1024));
  return Buffer.concat(parts);
}

// A real zstd frame with a small tar, then skippable frames up to the listed size: zstd reads past them and the
// archive still opens, while the holes inside the skippable frames cost no space.
export function tarZst(files, size) {
  if (!zlib.zstdCompressSync) return null;
  const frame = zlib.zstdCompressSync(tar(files));
  const chunks = [{ offset: 0, buf: frame }];
  const MAX = 0xffff0000;
  let at = frame.length;
  while (size - at >= 8) {
    const n = Math.min(size - at - 8, MAX);
    const h = Buffer.alloc(8);
    h.writeUInt32LE(0x184d2a50, 0);
    h.writeUInt32LE(n, 4);
    chunks.push({ offset: at, buf: h });
    at += 8 + n;
  }
  return layout(Math.max(at, frame.length), ...chunks);
}

// Enough of ISO 9660 for tools to recognise the image: the primary volume descriptor and its terminator.
export function iso(label, size) {
  const pvd = Buffer.alloc(2048);
  pvd[0] = 1;
  pvd.write('CD001', 1, 'ascii');
  pvd[6] = 1;
  pvd.write(label.toUpperCase().replace(/[^A-Z0-9_]/g, '_').slice(0, 32).padEnd(32, ' '), 40, 'ascii');
  const blocks = Math.floor(size / 2048);
  pvd.writeUInt32LE(blocks, 80);
  pvd.writeUInt32BE(blocks, 84);
  pvd.writeUInt16LE(2048, 128);
  pvd.writeUInt16BE(2048, 130);
  const term = Buffer.alloc(2048);
  term[0] = 255;
  term.write('CD001', 1, 'ascii');
  term[6] = 1;
  return layout(Math.max(size, 36864), { offset: 32768, buf: pvd }, { offset: 34816, buf: term });
}

// An empty SQLite database of one page; the header names one page, so the rest of the file is ignored.
export function sqlite(size) {
  const page = Buffer.alloc(4096);
  page.write('SQLite format 3\0', 0, 'latin1');
  page.writeUInt16BE(4096, 16);
  page[18] = 1;
  page[19] = 1;
  page[21] = 64;
  page[22] = 32;
  page[23] = 32;
  page.writeUInt32BE(1, 24);
  page.writeUInt32BE(1, 28);
  page.writeUInt32BE(4, 44);
  page.writeUInt32BE(1, 56);
  page.writeUInt32BE(1, 92);
  page.writeUInt32BE(3046000, 96);
  page[100] = 0x0d;
  page.writeUInt16BE(4096, 105);
  return layout(Math.max(size, 4096), { offset: 0, buf: page });
}

// A one-page PDF with the given lines. Comment lines pad it to size before the cross-reference table,
// which keeps every object offset valid.
export function pdf(title, lines, size) {
  const esc = (s) => s.replace(/[\\()]/g, (c) => '\\' + c);
  const text = ['BT /F1 20 Tf 72 760 Td', `(${esc(title)}) Tj`, '/F1 11 Tf 0 -34 Td 14 TL']
    .concat(lines.map((l) => `(${esc(l)}) '`), 'ET').join('\n');
  const objs = [
    '<< /Type /Catalog /Pages 2 0 R >>',
    '<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>',
    `<< /Length ${Buffer.byteLength(text, 'latin1')} >>\nstream\n${text}\nendstream`,
    '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>',
  ];
  let body = '%PDF-1.4\n%\xe2\xe3\xcf\xd3\n';
  const offsets = objs.map((o, i) => {
    const at = Buffer.byteLength(body, 'latin1');
    body += `${i + 1} 0 obj\n${o}\nendobj\n`;
    return at;
  });
  const tail = (xrefAt) => `xref\n0 ${objs.length + 1}\n0000000000 65535 f \n`
    + offsets.map((o) => `${String(o).padStart(10, '0')} 00000 n \n`).join('')
    + `trailer\n<< /Size ${objs.length + 1} /Root 1 0 R >>\nstartxref\n${xrefAt}\n%%EOF\n`;
  const fixed = Buffer.byteLength(body, 'latin1') + Buffer.byteLength(tail(String(size).padStart(10, '0')), 'latin1');
  let pad = Math.max(0, size - fixed);
  const line = '%' + ' '.repeat(78) + '\n';
  while (pad >= line.length) { body += line; pad -= line.length; }
  if (pad > 1) { body += '%' + ' '.repeat(pad - 2) + '\n'; pad = 0; }
  const xrefAt = Buffer.byteLength(body, 'latin1');
  let out = body + tail(String(xrefAt).padStart(10, '0'));
  if (pad === 1) out += '\n';
  return Buffer.from(out, 'latin1');
}

const xml = (s) => '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n' + s;
const xesc = (s) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
const NS = {
  ct: 'http://schemas.openxmlformats.org/package/2006/content-types',
  rel: 'http://schemas.openxmlformats.org/package/2006/relationships',
  r: 'http://schemas.openxmlformats.org/officeDocument/2006/relationships',
  doc: 'http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument',
  thumb: 'http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail',
  w: 'http://schemas.openxmlformats.org/wordprocessingml/2006/main',
  s: 'http://schemas.openxmlformats.org/spreadsheetml/2006/main',
  p: 'http://schemas.openxmlformats.org/presentationml/2006/main',
  a: 'http://schemas.openxmlformats.org/drawingml/2006/main',
};
const rels = (list) => xml(`<Relationships xmlns="${NS.rel}">${list.map(([id, type, target]) => `<Relationship Id="${id}" Type="${type}" Target="${target}"/>`).join('')}</Relationships>`);
const types = (overrides) => xml(`<Types xmlns="${NS.ct}"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="jpeg" ContentType="image/jpeg"/>${overrides.map(([part, type]) => `<Override PartName="${part}" ContentType="${type}"/>`).join('')}</Types>`);
const OFFICE = 'application/vnd.openxmlformats-officedocument.';

function wordParts(title, lines) {
  const para = (t, style) => `<w:p>${style ? `<w:pPr><w:pStyle w:val="${style}"/></w:pPr>` : ''}<w:r><w:t xml:space="preserve">${xesc(t)}</w:t></w:r></w:p>`;
  return {
    main: 'word/document.xml',
    types: [['/word/document.xml', OFFICE + 'wordprocessingml.document.main+xml']],
    parts: { 'word/document.xml': xml(`<w:document xmlns:w="${NS.w}"><w:body>${para(title, 'Title')}${lines.map((l) => para(l)).join('')}</w:body></w:document>`) },
  };
}

function sheetParts(title, rows) {
  const col = (i) => String.fromCharCode(65 + i);
  const cell = (v, r, c) => typeof v === 'number'
    ? `<c r="${col(c)}${r}"><v>${v}</v></c>`
    : `<c r="${col(c)}${r}" t="inlineStr"><is><t>${xesc(v)}</t></is></c>`;
  const data = rows.map((row, r) => `<row r="${r + 1}">${row.map((v, c) => cell(v, r + 1, c)).join('')}</row>`).join('');
  return {
    main: 'xl/workbook.xml',
    types: [['/xl/workbook.xml', OFFICE + 'spreadsheetml.sheet.main+xml'], ['/xl/worksheets/sheet1.xml', OFFICE + 'spreadsheetml.worksheet+xml']],
    parts: {
      'xl/workbook.xml': xml(`<workbook xmlns="${NS.s}" xmlns:r="${NS.r}"><sheets><sheet name="${xesc(title.slice(0, 31))}" sheetId="1" r:id="rId1"/></sheets></workbook>`),
      'xl/_rels/workbook.xml.rels': rels([['rId1', 'http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet', 'worksheets/sheet1.xml']]),
      'xl/worksheets/sheet1.xml': xml(`<worksheet xmlns="${NS.s}"><sheetData>${data}</sheetData></worksheet>`),
    },
  };
}

function slideParts(title, lines) {
  const R = 'http://schemas.openxmlformats.org/officeDocument/2006/relationships/';
  const tree = (shapes) => `<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>${shapes}</p:spTree></p:cSld>`;
  const box = (id, y, h, size, text) => `<p:sp><p:nvSpPr><p:cNvPr id="${id}" name="Text ${id}"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="457200" y="${y}"/><a:ext cx="8229600" cy="${h}"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/>${text.map((t) => `<a:p><a:r><a:rPr lang="de-AT" sz="${size}"/><a:t>${xesc(t)}</a:t></a:r></a:p>`).join('')}</p:txBody></p:sp>`;
  const ns = `xmlns:a="${NS.a}" xmlns:r="${NS.r}" xmlns:p="${NS.p}"`;
  const theme = `<a:theme ${`xmlns:a="${NS.a}"`} name="Fixture"><a:themeElements><a:clrScheme name="Fixture"><a:dk1><a:srgbClr val="000000"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="1F2937"/></a:dk2><a:lt2><a:srgbClr val="F3F4F6"/></a:lt2>${['FCC419', '4C6EF5', '40C057', 'FA5252', '7950F2', '15AABF'].map((c, i) => `<a:accent${i + 1}><a:srgbClr val="${c}"/></a:accent${i + 1}>`).join('')}<a:hlink><a:srgbClr val="4C6EF5"/></a:hlink><a:folHlink><a:srgbClr val="7950F2"/></a:folHlink></a:clrScheme><a:fontScheme name="Fixture"><a:majorFont><a:latin typeface="Arial"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont><a:minorFont><a:latin typeface="Arial"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme><a:fmtScheme name="Fixture"><a:fillStyleLst>${'<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>'.repeat(3)}</a:fillStyleLst><a:lnStyleLst>${'<a:ln><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>'.repeat(3)}</a:lnStyleLst><a:effectStyleLst>${'<a:effectStyle><a:effectLst/></a:effectStyle>'.repeat(3)}</a:effectStyleLst><a:bgFillStyleLst>${'<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>'.repeat(3)}</a:bgFillStyleLst></a:fmtScheme></a:themeElements></a:theme>`;
  const map = '<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>';
  return {
    main: 'ppt/presentation.xml',
    types: [
      ['/ppt/presentation.xml', OFFICE + 'presentationml.presentation.main+xml'],
      ['/ppt/slides/slide1.xml', OFFICE + 'presentationml.slide+xml'],
      ['/ppt/slideLayouts/slideLayout1.xml', OFFICE + 'presentationml.slideLayout+xml'],
      ['/ppt/slideMasters/slideMaster1.xml', OFFICE + 'presentationml.slideMaster+xml'],
      ['/ppt/theme/theme1.xml', OFFICE + 'theme+xml'],
    ],
    parts: {
      'ppt/presentation.xml': xml(`<p:presentation ${ns}><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst><p:sldIdLst><p:sldId id="256" r:id="rId2"/></p:sldIdLst><p:sldSz cx="9144000" cy="5143500"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`),
      'ppt/_rels/presentation.xml.rels': rels([['rId1', R + 'slideMaster', 'slideMasters/slideMaster1.xml'], ['rId2', R + 'slide', 'slides/slide1.xml'], ['rId3', R + 'theme', 'theme/theme1.xml']]),
      'ppt/slides/slide1.xml': xml(`<p:sld ${ns}>${tree(box(2, 457200, 914400, 3600, [title]) + box(3, 1600200, 3200400, 2000, lines))}</p:sld>`),
      'ppt/slides/_rels/slide1.xml.rels': rels([['rId1', R + 'slideLayout', '../slideLayouts/slideLayout1.xml']]),
      'ppt/slideLayouts/slideLayout1.xml': xml(`<p:sldLayout ${ns} type="blank">${tree('')}<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`),
      'ppt/slideLayouts/_rels/slideLayout1.xml.rels': rels([['rId1', R + 'slideMaster', '../slideMasters/slideMaster1.xml']]),
      'ppt/slideMasters/slideMaster1.xml': xml(`<p:sldMaster ${ns}>${tree('')}${map}<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst></p:sldMaster>`),
      'ppt/slideMasters/_rels/slideMaster1.xml.rels': rels([['rId1', R + 'slideLayout', '../slideLayouts/slideLayout1.xml'], ['rId2', R + 'theme', '../theme/theme1.xml']]),
      'ppt/theme/theme1.xml': xml(theme),
    },
  };
}

// docx, xlsx and pptx with real content and, when one is given, the preview picture Office files carry.
export function office(ext, { title, lines, rows, thumbnail, time, size }) {
  const kind = { docx: () => wordParts(title, lines), xlsx: () => sheetParts(title, rows), pptx: () => slideParts(title, lines) }[ext];
  if (!kind) return null;
  const k = kind();
  const top = [['rId1', NS.doc, k.main]];
  if (thumbnail) top.push(['rId2', NS.thumb, 'docProps/thumbnail.jpeg']);
  const parts = { '[Content_Types].xml': types(k.types), '_rels/.rels': rels(top), ...k.parts };
  const entries = Object.entries(parts).map(([name, s]) => ({ name, data: Buffer.from(s, 'utf8'), time }));
  if (thumbnail) entries.push({ name: 'docProps/thumbnail.jpeg', data: thumbnail, time });
  return zip(entries, { size });
}
