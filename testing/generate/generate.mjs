// node testing/generate/generate.mjs [--from prototype.json] [--root /fixture] [--clean] [--no-media]
// Builds the prototype's server as a real tree: the disks, the cache, the flash drive and the remotes under
// <root>/mnt, <root>/boot and <root>/remotes, emhttp's files under <root>/var/local/emhttp and <root>/proc/mdstat.
// /mnt/user is left empty, since mergerfs puts it together from the disks the way Unraid's shfs does.
import { chmodSync, chownSync, closeSync, ftruncateSync, lchownSync, linkSync, lutimesSync, mkdirSync, openSync, readFileSync, rmSync, symlinkSync, utimesSync, writeFileSync, writeSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { parseArgs } from 'node:util';
import { iso, office, pdf, sqlite, tarZst, zip } from './formats.mjs';
import { audio, dimsOf, hasFfmpeg, id3Padded, image, matroskaVoid, secondsOf, video } from './media.mjs';
import { emhttp } from './emhttp.mjs';

const here = import.meta.dirname;
const { values: opt } = parseArgs({
  options: {
    from: { type: 'string', default: join(here, '..', 'fixture', 'prototype.json') },
    root: { type: 'string', default: '/fixture' },
    clean: { type: 'boolean', default: false },
    'no-media': { type: 'boolean', default: false },
  },
});

// Files of many gigabytes only cost nothing where the file system keeps holes, which is the Linux container.
if (process.platform !== 'linux') {
  console.error('The generator runs on Linux, inside testing/container.');
  process.exit(2);
}

const exp = JSON.parse(readFileSync(opt.from, 'utf8'));
const root = opt.root;
const asRoot = process.getuid?.() === 0;
const IDS = { nobody: 99, users: 100, root: 0 };
const media = !opt['no-media'] && hasFfmpeg();
const stats = { files: 0, links: 0, symlinks: 0, dirs: 0, bytes: 0, media: 0 };

// The disks and the cache show through /mnt/user, which is where the tests look at a file.
const userPath = (p) => p.replace(/^\/mnt\/(cache|disk\d+)(?=\/|$)/, '/mnt/user');
const deviations = [];
const deviate = (path, kind, note) => deviations.push({ path: userPath(path), kind, note });

if (opt.clean) for (const d of ['mnt', 'boot', 'remotes', 'var', 'proc']) rmSync(join(root, d), { recursive: true, force: true });

const child = (n, name) => (n.children || []).find((c) => c.name === name);
const nodeAt = (path) => path.split('/').filter(Boolean).reduce((n, part) => n && child(n, part), exp.tree);

function writeLayout(path, { length, chunks }) {
  const fd = openSync(path, 'w');
  try {
    for (const c of chunks) writeSync(fd, c.buf, 0, c.buf.length, c.offset);
    ftruncateSync(fd, length);
  } finally {
    closeSync(fd);
  }
}

// A real file, then a hole up to the listed size, and the format's own way of growing where it has one.
function grown(buf, size, tail) {
  const chunks = [{ offset: 0, buf }];
  const filler = size > buf.length && tail?.(size - buf.length);
  if (filler) chunks.push({ offset: buf.length, buf: filler });
  return { length: Math.max(size, buf.length), chunks };
}

// Text keeps the prototype's lines. Short padding goes in as spaces before the last line break, so the line
// count stays the same; a log much smaller than its size grows by repeating its lines, as a real log does.
function textLayout(n, path, size) {
  let text = Buffer.from(n.text, 'utf8');
  const exact = () => ({ length: text.length, chunks: [{ offset: 0, buf: text }] });
  if (text.length > size) {
    deviate(path, 'size', `text is ${text.length} bytes, listed as ${size}`);
    return exact();
  }
  // Spaces after a file name would become part of the name for sha256sum.
  if (n.name.endsWith('.sha256') && text.length < size) {
    deviate(path, 'size', `checksum file stays ${text.length} bytes, listed as ${size}`);
    return exact();
  }
  if (size - text.length > 64 * 1024 && /\n./.test(n.text)) {
    const reps = Math.floor(size / text.length);
    text = Buffer.concat(Array(reps).fill(text));
    deviate(path, 'lines', `lines repeated ${reps} times to reach ${size} bytes`);
  }
  const end = text.at(-1) === 0x0a ? text.length - 1 : text.length;
  const buf = Buffer.concat([text.subarray(0, end), Buffer.alloc(size - text.length, 0x20), text.subarray(end)]);
  return { length: buf.length, chunks: [{ offset: 0, buf }] };
}

const stem = (name) => name.replace(/\.[^.]+$/, '');
const ext = (name) => (name.includes('.') ? name.split('.').pop().toLowerCase() : '');

function zipEntries(list, prefix = '') {
  return list.flatMap((z) => {
    const name = prefix + z.name;
    if (z.dir) return [{ name: name + '/', dir: true, time: z.time }, ...zipEntries(z.children || [], name + '/')];
    const size = Math.floor(z.size || 0);
    const pic = media && ext(z.name) === 'jpg' ? image(name, dimsOf(z.cols?.dimensions, [1600, 1200]), size) : null;
    return [{ name, data: pic || Buffer.from(`${z.name}\n`), size, time: z.time }];
  });
}

function fileLayout(n, path) {
  const size = Math.floor(n.size || 0);
  const e = ext(n.name);
  const cols = n.cols || {};
  // Office keeps a few bytes of owner data in its lock files, which no reader opens.
  if (!size || n.name.startsWith('~$')) return { length: size, chunks: [] };
  if (n.text != null) return textLayout(n, path, size);
  if (e === 'zip') return zip(zipEntries(n.zip || []), { size });
  if (n.name.endsWith('.tar.zst')) {
    const l = tarZst([{ name: stem(stem(n.name)) + '/README', data: Buffer.from(`${n.name}\nBackup aus dem Prototyp.\n`), time: n.time }], size);
    if (l) return l;
  }
  if (e === 'iso') return iso(stem(n.name), size);
  if (['db', 'sqlite'].includes(e)) return sqlite(size);
  if (e === 'pdf') return { length: size, chunks: [{ offset: 0, buf: pdf(stem(n.name), [n.name, `Pfad: ${userPath(path)}`, 'Erstellt für die Tests von CannonCommander.'], size) }] };
  if (['docx', 'xlsx', 'pptx'].includes(e)) {
    const thumbnail = media ? image(n.name, [256, 362], 64 * 1024) : null;
    const rows = [['Posten', 'Betrag'], ['Miete', 9600], ['Strom', 840], ['Internet', 420], ['Summe', 10860]];
    const l = office(e, { title: stem(n.name), lines: [`Datei ${n.name}`, 'Erstellt für die Tests von CannonCommander.'], rows, thumbnail, time: n.time, size });
    if (l.length > size) deviate(path, 'size', `office file is ${l.length} bytes, listed as ${size}`);
    return l;
  }
  const made = (buf, what, tail) => {
    stats.media++;
    if (buf.length > size) deviate(path, 'size', `${what} is ${buf.length} bytes, listed as ${size}`);
    return grown(buf, size, tail);
  };
  if (media && n.format === 'image' && e !== 'heic') {
    const pic = image(n.name, dimsOf(cols.dimensions), size);
    if (pic) return made(pic, 'picture');
  }
  if (media && n.format === 'video') {
    const film = video(n.name, dimsOf(cols.dimensions, [1280, 720]), secondsOf(cols.duration), cols);
    if (film) return made(film, 'film', matroskaVoid);
  }
  if (media && n.format === 'audio') {
    const song = audio(n.name, e, secondsOf(cols.duration), cols);
    const tagged = song && e === 'mp3' && id3Padded(song, size);
    if (tagged) { stats.media++; return tagged; }
    if (song) return made(song, 'audio');
  }
  return { length: size, chunks: [] };
}

const owned = [];
const inodes = new Map();

function build(n, path, dev) {
  const at = join(root, path);
  if (n.dir) {
    mkdirSync(at, { recursive: true });
    stats.dirs++;
    for (const c of n.children || []) build(c, `${path}/${c.name}`, dev);
    // mergerfs shows a folder that sits on several disks with the times of the first one, so every disk gets
    // the times the prototype gives the folder in /mnt/user.
    owned.push([at, nodeAt(userPath(path)) || n]);
    return;
  }
  mkdirSync(dirname(at), { recursive: true });
  if (n.link) {
    symlinkSync(n.link, at);
    stats.symlinks++;
    owned.push([at, n, true]);
    return;
  }
  // Files sharing an inode in the prototype are hard links on the same disk, as Unraid keeps them.
  const key = n.ino && `${dev}:${n.ino}`;
  if (key && inodes.has(key)) {
    linkSync(inodes.get(key), at);
    stats.links++;
    return;
  }
  const l = fileLayout(n, path);
  writeLayout(at, l);
  if (key) inodes.set(key, at);
  stats.files++;
  stats.bytes += l.length;
  owned.push([at, n]);
}

const mnt = child(exp.tree, 'mnt');
for (const v of mnt.children.filter((c) => c.name !== 'user')) build(v, `/mnt/${v.name}`, v.name);
mkdirSync(join(root, 'mnt', 'user'), { recursive: true });
build(child(exp.tree, 'boot'), '/boot', 'boot');
build(child(exp.tree, 'remotes'), '/remotes', 'remotes');

for (const [path, text] of Object.entries(emhttp(exp, join(here, '..', 'emhttp')))) {
  mkdirSync(dirname(join(root, path)), { recursive: true });
  writeFileSync(join(root, path), text);
}
deviate('/boot/config/shares', 'extra', 'share settings Unraid keeps on the flash drive');

// Owners, rights and times last and children before their folder, so writing into a folder does not move its time again.
for (const [at, n, isLink] of owned) {
  const t = new Date(n.time);
  if (isLink) {
    if (asRoot) lchownSync(at, IDS[n.owner] ?? 99, IDS[n.group] ?? 100);
    lutimesSync(at, t, t);
    continue;
  }
  if (asRoot) chownSync(at, IDS[n.owner] ?? 99, IDS[n.group] ?? 100);
  chmodSync(at, parseInt(n.mode || (n.dir ? '0775' : '0664'), 8));
  utimesSync(at, t, t);
}

writeFileSync(join(root, 'fixture.json'), JSON.stringify({ from: opt.from, now: exp.now, media, stats, deviations }, null, 1) + '\n');
console.log(`${root}: ${stats.files} files, ${stats.links} hard links, ${stats.symlinks} symlinks, ${stats.dirs} folders, `
  + `${(stats.bytes / 2 ** 30).toFixed(1)} GiB listed, ${stats.media} made by ffmpeg, ${deviations.length} deviations`);
