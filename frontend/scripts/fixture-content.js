// The content endpoints of the fixture: text, bytes, pictures and pages of the prototype's made-up files.
// Pictures and pages are the prototype's own drawings, so the viewer shows what the prototype shows; office files
// come from the test tree's generator, so the built-in office previews have real files to read.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';
import { office } from '../../testing/generate/formats.mjs';
import { clock } from './fixture-clock.js';

const repo = join(import.meta.dirname, '..', '..');
const js = (f) => readFileSync(join(repo, 'prototype', 'src', 'js', f), 'utf8');
const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]);
const KB = 1024, MB = KB * 1024;
// The names media libraries give a folder's picture, the first one found counting.
export const FOLDER_PICTURES = ['poster.jpg', 'folder.jpg', 'cover.jpg'];

// The prototype's drawing helpers and document pages, run against the same tree the listing comes from.
function drawings(FS) {
  const CC = { css: () => {}, prefs: {}, actions: {} };
  const ctx = vm.createContext({ CC, FS, esc, g: () => '', KB, MB, fmtSize: (b) => `${b}`, winStack: [], comboOf: () => '', $: () => ({ addEventListener() {}, textContent: '' }), $$: () => [], Files: {}, frow: () => '', info: () => '', seg: () => '', sw: () => '', swVal: () => false, segVal: () => '', wireControls: () => {}, closeBtn: {} });
  vm.runInContext(js('features/c-00-kit.js'), ctx);
  Object.assign(CC.ck, { R: {} });
  vm.runInContext(js('features/c-11-docs.js'), ctx);
  return CC.ck;
}

const exported = JSON.parse(readFileSync(join(repo, 'testing', 'fixture', 'prototype.json'), 'utf8'));
function exportedAt(path) {
  let n = exported.tree;
  for (const part of path.split('/').filter(Boolean)) {
    const zip = part.endsWith('::'), name = zip ? part.slice(0, -2) : part;
    const next = (zip ? n.children : n.children || n.zip)?.find((k) => k.name === name);
    if (!next) return null;
    n = zip ? { ...next, children: next.zip } : next;
  }
  return n;
}

const send = (res, type, body, headers = {}) => {
  res.setHeader('Content-Type', type);
  for (const [k, v] of Object.entries(headers)) res.setHeader(k, v);
  res.end(body);
};
const json = (res, body) => send(res, 'application/json', JSON.stringify(body));
const fail = (res, status, title) => {
  res.statusCode = status;
  send(res, 'application/problem+json', JSON.stringify({ type: 'about:blank', title, status }));
};

const ISO = {
  'ubuntu-24.04.3-desktop-amd64.iso': ['Ubuntu 24.04.3 LTS amd64', ['casper/', 'boot/', 'EFI/', 'dists/', 'pool/', 'md5sum.txt']],
  'Win11_24H2_German_x64.iso': ['CCCOMA_X64FRE_DE-DE_DV9', ['boot/', 'efi/', 'sources/', 'support/', 'autorun.inf', 'setup.exe']],
  'debian-13.1.0-amd64-netinst.iso': ['Debian 13.1.0 amd64 n', ['boot/', 'dists/', 'EFI/', 'install.amd/', 'pool/', 'md5sum.txt']],
};

const PAGE_CSS = readFileSync(join(import.meta.dirname, '..', 'src', 'features', 'viewer', 'viewer.css'), 'utf8')
  .split('\n').filter((l) => l.startsWith('.cv-paper')).join('\n');

export function contentRoutes(FS) {
  const ck = drawings(FS);
  const texts = new Map();
  const nodeAt = (path) => FS.nodeAt(path);
  const parent = (path) => path.slice(0, path.lastIndexOf('/')) || '/';
  const nameOf = (path) => path.slice(path.lastIndexOf('/') + 1);
  const textOf = (path, n) => texts.get(path) ?? n?.text ?? exportedAt(path)?.text ?? (n && n.size === 0 ? '' : null);

  // The prototype's page drawings come out of its pager as markup; each page sits in a cv-paper of its own.
  function pagesOf(n) {
    let html = '';
    const host = { set innerHTML(v) { html = v; }, get innerHTML() { return html; }, addEventListener() {} };
    (FS.ext(n.name) === 'pdf' ? ck.R.pdf : ck.R.office)(host, n, { compact: false, folder: '' });
    const start = html.indexOf('<div class="cv-pages">');
    if (start < 0) return [];
    const body = html.slice(start + '<div class="cv-pages">'.length).replace(/<\/div><\/div>\s*$/, '');
    return body.split('<div class="cv-paper" data-page="').slice(1).map((p) => p.replace(/^\d+"><div class="pg">/, '').replace(/<\/div><\/div>$/, ''));
  }
  const pageSvg = (inner) => `<svg xmlns="http://www.w3.org/2000/svg" width="1240" height="1754" viewBox="0 0 1240 1754"><foreignObject width="1240" height="1754">`
    + `<div xmlns="http://www.w3.org/1999/xhtml" style="width:1240px;height:1754px"><style>${PAGE_CSS}</style><div class="cv-paper" style="width:1240px;border-radius:0"><div class="pg">`
    + inner.replace(/<br>/g, '<br/>').replace(/<hr>/g, '<hr/>') + '</div></div></div></foreignObject></svg>';

  const picture = (n, folder) => {
    const [w, h] = ck.dims(n, folder);
    return ck.picture(n, folder).replace('<svg ', `<svg width="${w}" height="${h}" `);
  };
  const filmOf = (n, folder) => { const f = FS.split(folder).at(-1) || ''; return /\(\d{4}\)/.test(f) ? f : n.name; };

  // The archives the prototype lists without a zip of their own, as a little tree to walk through "::" paths.
  function archiveTree(n) {
    if (n.zip) return null;
    let flat;
    if (/^appdata-.*\.tar/.test(n.name)) {
      flat = [];
      FS.walk((k, p) => flat.push({ path: 'appdata' + p, dir: !!k.dir, size: k.size || 0 }), FS.nodeAt('/mnt/user/appdata'));
    } else if (/^flash-/.test(n.name)) {
      flat = ['bzimage', 'bzroot', 'bzmodules', 'bzfirmware', 'config/', 'config/ident.cfg', 'config/go', 'config/super.dat', 'config/disk.cfg', 'config/plugins/', 'syslinux/syslinux.cfg']
        .map((p) => ({ path: p.replace(/\/$/, ''), dir: p.endsWith('/'), size: p.endsWith('/') ? 0 : Math.round(ck.rng(p)() * (p.startsWith('bz') ? 140 * MB : 8 * KB)) }));
    } else if (ISO[n.name]) {
      flat = ISO[n.name][1].map((p) => ({ path: p.replace(/\/$/, ''), dir: p.endsWith('/'), size: 0 }));
    } else return null;
    const root = { children: [] };
    for (const e of flat) {
      let at = root;
      const parts = e.path.split('/');
      parts.forEach((part, k) => {
        let next = at.children.find((c) => c.name === part);
        if (!next) at.children.push(next = { name: part, dir: k < parts.length - 1 || e.dir, size: k === parts.length - 1 ? e.size : 0, children: [] });
        at = next;
      });
    }
    return root;
  }

  return {
    // Lists inside the archives above; the plain listing answers everything else.
    list(path) {
      const at = path.indexOf('::');
      if (at < 0) return null;
      const n = nodeAt(path.slice(0, at));
      const tree = n && archiveTree(n);
      if (!tree) return null;
      let dir = tree;
      for (const part of path.slice(at + 2).split('/').filter(Boolean)) dir = dir?.children.find((c) => c.name === part);
      if (!dir) return null;
      return { path, items: dir.children.map((c) => ({ name: c.name, dir: c.dir, size: c.size, mtime: new Date(n.time).toISOString(), mode: '0644', owner: 'root', group: 'root' })) };
    },
    handle(url, req, res) {
      const path = url.searchParams.get('path') || '/';
      const n = nodeAt(path);
      const route = url.pathname.replace('/api/v1', '');
      if (route === '/system') return json(res, { features: { office: { kind: 'none' } } }), true;
      if (route === '/fs/touch' && req.method === 'POST') {
        let body = '';
        req.on('data', (c) => { body += c; });
        req.on('end', () => {
          const p = JSON.parse(body || '{}').path || '';
          const dir = nodeAt(parent(p));
          if (!dir?.children || nodeAt(p)) return fail(res, 409, 'Exists');
          dir.children.push({ name: nameOf(p), size: 0, time: clock.now(), owner: 'nobody', group: 'users', mode: '0664' });
          texts.set(p, '');
          json(res, { path: p });
        });
        return true;
      }
      if (route === '/fs/file' && req.method === 'PUT') {
        let body = '';
        req.on('data', (c) => { body += c; });
        req.on('end', () => {
          const dir = n ? null : nodeAt(parent(path));
          if (!n && !dir?.children) return fail(res, 409, 'The folder for a new file is missing');
          const { text = '' } = JSON.parse(body || '{}');
          const file = n || { name: nameOf(path), owner: 'nobody', group: 'users', mode: '0664' };
          if (!n) dir.children.push(file);
          texts.set(path, text);
          Object.assign(file, { size: Buffer.byteLength(text), time: clock.now(), text });
          if (!n) res.statusCode = 201;
          json(res, { size: file.size, mtime: new Date(file.time).toISOString() });
        });
        return true;
      }
      if (!['/fs/text', '/fs/hex', '/fs/raw', '/fs/thumb', '/fs/page', '/fs/stat'].includes(route)) return false;
      if (!n) return fail(res, 404, 'Not found'), true;
      const folder = parent(path), e = FS.ext(n.name), kind = FS.kindOf(n);

      if (route === '/fs/stat') {
        const image = e === 'iso'
          ? { format: 'ISO 9660 mit UDF', label: ISO[n.name]?.[0] || n.name.replace(/\.iso$/, '').toUpperCase(), boot: 'BIOS und UEFI' }
          : e === 'img'
            ? { format: 'Rohes Disk-Image', used: Math.round(n.size * 0.41), parts: n.name.startsWith('vm-')
              ? [['EFI', 100 * MB], ['MSR', 16 * MB], ['Windows, NTFS', n.size - 866 * MB], ['Wiederherstellung', 750 * MB]].map(([label, size]) => ({ label, size: Math.round(size) }))
              : [{ label: n.name === 'docker.img' ? 'Docker, Btrfs' : 'libvirt, Btrfs', size: Math.round(n.size) }] }
            : undefined;
        return json(res, { path, size: Math.round(n.size || 0), mtime: new Date(n.time).toISOString(), ...(image ? { image } : {}) }), true;
      }
      if (route === '/fs/text') {
        const text = textOf(path, n);
        if (text == null) return fail(res, 415, 'Binary'), true;
        const max = Number(url.searchParams.get('max')) || 1048576;
        const buf = Buffer.from(text);
        const cut = buf.length > max;
        return json(res, { path, size: buf.length, mtime: new Date(n.time).toISOString(), encoding: 'utf-8', eol: 'lf', text: cut ? buf.subarray(0, max).toString() : text, ...(cut ? { truncated: true } : {}) }), true;
      }
      if (route === '/fs/hex') {
        const offset = Number(url.searchParams.get('offset')) || 0, length = Number(url.searchParams.get('length')) || 4096;
        const r = ck.rng(n.name);
        const magic = /\.(db|sqlite)$/.test(n.name) ? 'SQLite format 3\0' : n.name === 'bzimage' ? 'MZ\x90\0\x03\0\0\0' : n.name === 'bzroot' ? '070701' : '';
        const bytes = Array.from({ length: offset + length }, (_, k) => (k < magic.length ? magic.charCodeAt(k) : r() < 0.3 ? 0 : Math.floor(r() * 256))).slice(offset);
        return json(res, { path, size: Math.round(n.size || 0), offset, data: Buffer.from(bytes).toString('base64') }), true;
      }
      if (route === '/fs/thumb') {
        const large = url.searchParams.get('size') === 'large';
        // A film or series folder shows a tall poster with its title, an album a square cover.
        if (n.dir) {
          const pic = FS.kids(n).find((k) => FOLDER_PICTURES.includes(k.name.toLowerCase()));
          if (!pic) return (res.statusCode = 204, res.end(), true);
          const square = pic.name === 'cover.jpg' || path.startsWith('/mnt/user/musik');
          return send(res, 'image/svg+xml', square ? ck.scene(n.name, { w: 1000, h: 1000, title: n.name }) : ck.poster(n.name, 1000, 1500)), true;
        }
        if (kind === 'img') return send(res, 'image/svg+xml', picture(n, folder)), true;
        if (kind === 'video') return send(res, 'image/svg+xml', large ? ck.poster(filmOf(n, folder), 1600, 900, false) : ck.poster(n.name, 640, 360, false)), true;
        // No content instead of 404 keeps the browser console of the test runs clean; the app treats both alike.
        return (res.statusCode = 204, res.end(), true);
      }
      if (route === '/fs/page') {
        const pages = kind === 'pdf' || kind === 'office' ? pagesOf(n) : [];
        const k = (Number(url.searchParams.get('page')) || 1) - 1;
        if (!pages[k]) return fail(res, 404, 'No such page'), true;
        return send(res, 'image/svg+xml', pageSvg(pages[k]), { 'X-Page-Count': String(pages.length) }), true;
      }
      // fs/raw: pictures as the prototype draws them, office files built for the test tree, text as it is.
      if (kind === 'img') return send(res, 'image/svg+xml', picture(n, folder)), true;
      if (['docx', 'xlsx', 'pptx'].includes(e)) {
        const rows = [['Posten', 'Betrag'], ['Miete', 9600], ['Strom', 840], ['Internet', 420], ['Summe', 10860]];
        const made = office(e, { title: n.name.replace(/\.[^.]+$/, ''), lines: [`Datei ${n.name}`, 'Erstellt für die Tests von CannonCommander.'], rows, time: n.time, size: 0 });
        const buf = Buffer.alloc(made.length);
        for (const c of made.chunks) c.buf.copy(buf, c.offset);
        return send(res, 'application/octet-stream', buf), true;
      }
      const text = textOf(path, n);
      if (text != null) return send(res, 'text/plain; charset=utf-8', text), true;
      return (res.statusCode = 204, res.end(), true);
    },
  };
}
