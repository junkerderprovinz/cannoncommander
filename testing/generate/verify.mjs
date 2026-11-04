// node testing/generate/verify.mjs [--from prototype.json] [--root /fixture] [--list listing.txt]
// Checks the tree mergerfs puts together under <root>/mnt/user against the prototype's /mnt/user: every folder and
// file in the same place, with its size, owner, rights and time, hard links where the prototype has them.
import { lstatSync, readdirSync, readFileSync, readlinkSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { parseArgs } from 'node:util';

const here = import.meta.dirname;
const { values: opt } = parseArgs({
  options: {
    from: { type: 'string', default: join(here, '..', 'fixture', 'prototype.json') },
    root: { type: 'string', default: '/fixture' },
    list: { type: 'string' },
  },
});

const exp = JSON.parse(readFileSync(opt.from, 'utf8'));
const manifest = JSON.parse(readFileSync(join(opt.root, 'fixture.json'), 'utf8'));
const allowed = new Map(manifest.deviations.map((d) => [d.path, d]));
const IDS = { nobody: 99, users: 100, root: 0 };
const problems = [];
const lines = [];
let checked = 0;

const user = exp.tree.children.find((c) => c.name === 'mnt').children.find((c) => c.name === 'user');

function check(n, path) {
  const at = join(opt.root, path);
  let st;
  try {
    st = lstatSync(at);
  } catch {
    problems.push(`${path}: missing`);
    return;
  }
  checked++;
  const mode = (st.mode & 0o7777).toString(8).padStart(4, '0');
  lines.push(`${n.dir ? 'd' : n.link ? 'l' : '-'} ${mode} ${st.uid}:${st.gid} ${String(st.size).padStart(12)} ${new Date(st.mtimeMs).toISOString().slice(0, 16)} ${st.nlink} ${path}`);
  if (n.link) {
    if (!st.isSymbolicLink() || readlinkSync(at) !== n.link) problems.push(`${path}: not a link to ${n.link}`);
    return;
  }
  if (Boolean(n.dir) !== st.isDirectory()) problems.push(`${path}: ${n.dir ? 'should be a folder' : 'should be a file'}`);
  if (mode !== (n.mode || '').padStart(4, '0')) problems.push(`${path}: mode ${mode}, prototype ${n.mode}`);
  if (st.uid !== (IDS[n.owner] ?? 99) || st.gid !== (IDS[n.group] ?? 100)) problems.push(`${path}: owner ${st.uid}:${st.gid}, prototype ${n.owner}:${n.group}`);
  if (Math.abs(st.mtimeMs - n.time) > 1000) problems.push(`${path}: time ${new Date(st.mtimeMs).toISOString()}, prototype ${new Date(n.time).toISOString()}`);
  if (n.dir) {
    const want = (n.children || []).map((c) => c.name).sort();
    const have = readdirSync(at).sort();
    for (const name of have.filter((x) => !want.includes(x))) problems.push(`${path}/${name}: not in the prototype`);
    for (const c of n.children || []) check(c, `${path}/${c.name}`);
    return;
  }
  if (st.size !== Math.floor(n.size || 0) && !allowed.has(path)) problems.push(`${path}: ${st.size} bytes, prototype ${Math.floor(n.size || 0)}`);
  const links = n.ino ? 1 + countLinks(n.ino, n.dev) : 1;
  if (st.nlink !== links) problems.push(`${path}: ${st.nlink} hard links, prototype ${links}`);
}

function countLinks(ino, dev) {
  let n = -1;
  const walk = (k) => { if (!k.dir && k.ino === ino && k.dev === dev) n++; (k.children || []).forEach(walk); };
  walk(user);
  return n;
}

for (const c of user.children) check(c, `/mnt/user/${c.name}`);

if (opt.list) writeFileSync(opt.list, lines.join('\n') + '\n');
console.log(`${checked} entries under /mnt/user checked, ${manifest.deviations.length} known deviations, ${problems.length} problems`);
if (problems.length) {
  console.log(problems.join('\n'));
  process.exit(1);
}
