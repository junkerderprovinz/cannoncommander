// The files Unraid's emhttp writes, rebuilt for the prototype's server: the same keys and order as the anonymised
// files from a real Unraid 7.3 box in testing/emhttp, with the values of the prototype's disks and shares.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const KIB = 1024;

export function parseIni(text) {
  const out = {};
  let section = null;
  for (const line of text.split('\n')) {
    const head = /^\["?([^"\]]+)"?\]$/.exec(line.trim());
    if (head) { section = out[head[1]] = {}; continue; }
    const kv = /^([^=]+)="?(.*?)"?$/.exec(line.trim());
    if (kv && section) section[kv[1]] = kv[2];
  }
  return out;
}

const ini = (sections) => Object.entries(sections)
  .map(([name, keys]) => `["${name}"]\n` + Object.entries(keys).map(([k, v]) => `${k}="${v}"`).join('\n')).join('\n') + '\n';
const flat = (keys) => Object.entries(keys).map(([k, v]) => `${k}="${v}"`).join('\n') + '\n';
const kib = (bytes) => String(Math.floor(bytes / KIB));

export function emhttp(exp, templates) {
  const t = {
    disks: parseIni(readFileSync(join(templates, 'disks.ini'), 'utf8')),
    shares: parseIni(readFileSync(join(templates, 'shares.ini'), 'utf8')),
    var: parseIni('["var"]\n' + readFileSync(join(templates, 'var.ini'), 'utf8')).var,
    mdstat: readFileSync(join(templates, 'mdstat'), 'utf8'),
    cfg: readFileSync(join(templates, 'shares', 'appdata.cfg'), 'utf8'),
  };
  const { free, total } = exp.volumes;
  const { disks, spin, shares } = exp.unraid;
  const asleep = (d) => spin[d] === false;
  const parityKiB = Math.max(...disks.map((d) => total['/mnt/' + d])) / KIB;

  const diskSections = { parity: { ...t.disks.parity, device: 'sdb', id: 'FIXTURE_PARITY', idSb: 'FIXTURE_PARITY', size: String(parityKiB), sizeSb: String(parityKiB), sectors: String(parityKiB * 2), spundown: '0', temp: '34' } };
  disks.forEach((d, i) => {
    const n = i + 1, size = total['/mnt/' + d], left = free['/mnt/' + d];
    diskSections[d] = {
      ...t.disks.disk1, idx: String(n), name: d, device: 'sd' + String.fromCharCode(99 + i), id: `FIXTURE_DISK${n}`, idSb: `FIXTURE_DISK${n}`,
      size: kib(size), sizeSb: kib(size), sectors: String(size / 512), deviceSb: `md${n}p1`,
      spundown: asleep(d) ? '1' : '0', color: asleep(d) ? 'green-blink' : 'green-on', temp: asleep(d) ? '*' : String(31 + i),
      fsMountpoint: '/mnt/' + d, fsSize: kib(size), fsFree: kib(left), fsUsed: kib(size - left),
    };
  });
  const cacheSize = total['/mnt/cache'], cacheFree = free['/mnt/cache'];
  diskSections.cache = {
    ...t.disks.cache, device: 'nvme0n1', id: 'FIXTURE_CACHE', idSb: 'FIXTURE_CACHE', deviceSb: 'nvme0n1p1',
    size: kib(cacheSize), sizeSb: kib(cacheSize), sectors: String(cacheSize / 512), fsSize: kib(cacheSize), fsFree: kib(cacheFree), fsUsed: kib(cacheSize - cacheFree),
  };
  diskSections.flash = {
    ...t.disks.flash, device: 'sda', id: 'FIXTURE_FLASH', size: kib(total['/boot']), sectors: String(total['/boot'] / 512),
    fsSize: kib(total['/boot']), fsFree: kib(free['/boot']), fsUsed: kib(total['/boot'] - free['/boot']),
  };

  // Unraid names the share's storage with one word: only (cache alone), yes (cache, then array),
  // prefer (array, then cache) and no (array alone).
  const useCache = (s) => s.primary !== 'cache' ? 'no' : !s.secondary ? 'only' : s.mover === 'up' ? 'prefer' : 'yes';
  const userShares = exp.tree.children.find((c) => c.name === 'mnt').children.find((c) => c.name === 'user').children.filter((c) => c.dir).map((c) => c.name);
  const shareOf = (name) => shares[name] || { primary: 'array', disks };
  const sizeOn = (name) => {
    let sum = 0;
    const walk = (n) => { if (!n.dir) sum += n.size || 0; (n.children || []).forEach(walk); };
    walk(exp.tree.children.find((c) => c.name === 'mnt').children.find((c) => c.name === 'user').children.find((c) => c.name === name));
    return sum;
  };
  const template = t.shares.download || Object.values(t.shares)[0];
  const shareSections = {}, cfgs = {};
  for (const name of userShares) {
    const s = shareOf(name);
    const onArray = s.primary === 'array' || s.secondary === 'array';
    const room = (onArray ? s.disks : []).reduce((a, d) => a + free['/mnt/' + d], 0) + (s.primary === 'cache' ? cacheFree : 0);
    shareSections[name] = {
      ...template, name, nameOrig: name, include: onArray ? s.disks.join(',') : '', useCache: useCache(s), cachePool: s.primary === 'cache' ? 'cache' : '',
      free: kib(room), used: kib(sizeOn(name)), exclusive: s.primary === 'cache' && !s.secondary ? 'yes' : 'no', color: 'green-on',
    };
    cfgs[name] = t.cfg
      .replace(/^shareInclude=.*$/m, `shareInclude="${shareSections[name].include}"`)
      .replace(/^shareUseCache=.*$/m, `shareUseCache="${shareSections[name].useCache}"`)
      .replace(/^shareCachePool=.*$/m, `shareCachePool="${shareSections[name].cachePool}"`)
      .replace(/^shareWriteList=.*$/m, 'shareWriteList=""');
  }

  const v = {
    ...t.var, NAME: 'Fixture', COMMENT: 'CannonCommander test server', deviceCount: String(disks.length + 3),
    sbNumDisks: String(disks.length + 1), mdNumDisks: String(disks.length + 1), mdNumDisabled: '0', mdNumInvalid: '0', mdResyncSize: String(parityKiB),
    fsNumMounted: String(disks.length + 1), fsNumUnmountable: '0', fsUnmountableMask: '', shareCount: String(userShares.length),
  };

  // /proc/mdstat in Unraid's md driver lists every slot; the ones the prototype has no disk for are empty.
  const head = t.mdstat.slice(0, t.mdstat.indexOf('diskNumber.0='))
    .replace(/^sbNumDisks=.*$/m, `sbNumDisks=${disks.length + 1}`).replace(/^mdNumDisks=.*$/m, `mdNumDisks=${disks.length + 1}`)
    .replace(/^mdNumDisabled=.*$/m, 'mdNumDisabled=0').replace(/^mdNumInvalid=.*$/m, 'mdNumInvalid=0').replace(/^mdResyncSize=.*$/m, `mdResyncSize=${parityKiB}`);
  const slots = Math.max(...[...t.mdstat.matchAll(/^diskNumber\.(\d+)=/gm)].map((m) => +m[1])) + 1;
  const slot = (i) => {
    const sec = i === 0 ? diskSections.parity : diskSections[disks[i - 1]];
    const s = sec ? { name: i ? `md${i}p1` : '', size: sec.size, state: 7, status: 'DISK_OK', dev: sec.device, id: sec.id } : { name: '', size: 0, state: 0, status: 'DISK_NP', dev: '', id: '' };
    return [`diskNumber.${i}=${i}`, `diskName.${i}=${s.name}`, `diskSize.${i}=${s.size}`, `diskState.${i}=${s.state}`, `diskId.${i}=${s.id}`,
      `rdevNumber.${i}=${i}`, `rdevStatus.${i}=${s.status}`, `rdevName.${i}=${s.dev}`, `rdevOffset.${i}=${sec ? 64 : 0}`, `rdevSize.${i}=${s.size}`,
      `rdevId.${i}=${s.id}`, `rdevReads.${i}=0`, `rdevWrites.${i}=0`, `rdevNumErrors.${i}=0`].join('\n');
  };
  const mdstat = head + Array.from({ length: slots }, (_, i) => slot(i)).join('\n') + '\n';

  return {
    'var/local/emhttp/disks.ini': ini(diskSections),
    'var/local/emhttp/shares.ini': ini(shareSections),
    'var/local/emhttp/var.ini': flat(v),
    'proc/mdstat': mdstat,
    ...Object.fromEntries(Object.entries(cfgs).map(([n, c]) => [`boot/config/shares/${n}.cfg`, c])),
  };
}
