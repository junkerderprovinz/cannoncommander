import { beforeAll, describe, expect, it } from 'vitest';
import type { Provider, RemoteType } from '@/api/admin';
import { addMessages, setLanguage } from '@/i18n';
import catalog from '../../../scripts/fixture-remotes.json';
import de from './de.json';
import en from './en.json';
import { hasGlyph } from '@/core/glyphs';
import { applies, loadMarks, markOf, nameAndKind, optionHint, optionLabel, productName, providerName, providerPicker, remoteKind, splitOptions, suggestName } from './remotes';

beforeAll(async () => {
  addMessages('config', { de, en });
  setLanguage('en');
  await loadMarks();
});

const nextcloud: Provider = {
  id: 'nextcloud', backend: 'webdav', preset: { vendor: 'nextcloud' }, auth: 'apppassword', urlHint: 'https://cloud.example.com/remote.php/webdav/',
};
const option = (name: string, more: Partial<RemoteType['options'][number]> = {}) => ({
  name, help: '', type: 'string', required: false, essential: false, secret: false, advanced: false, exclusive: false, ...more,
});

// What the list of remotes called each provider before the picker took its tiles from GlimStone's storage catalog.
const SHOWN: Record<string, string> = {
  alias: 'Alias', archive: 'Archive files', azureblob: 'Azure Blob Storage', azurefiles: 'Azure Files',
  b2: 'Backblaze B2', box: 'Box', cache: 'Cache', ceph: 'Ceph', chunker: 'Split into pieces',
  cloudinary: 'Cloudinary', combine: 'Combined remotes', compress: 'Compressed', crypt: 'Encrypted', doi: 'DOI',
  drime: 'Drime', dropbox: 'Dropbox', fichier: '1Fichier', filefabric: 'Enterprise File Fabric', filelu: 'FileLu',
  filen: 'Filen', filescom: 'Files.com', ftp: 'FTP', garage: 'Garage', gcs: 'Google Cloud Storage',
  gdrive: 'Google Drive', gofile: 'Gofile', googlephotos: 'Google Photos', hasher: 'Checksums', hdfs: 'HDFS',
  hetznerobj: 'Hetzner Object Storage', hidrive: 'IONOS HiDrive', http: 'HTTP', huaweidrive: 'Huawei Drive',
  huaweiobs: 'Huawei Cloud OBS', icloud: 'iCloud Drive', idrivee2: 'IDrive e2', imagekit: 'ImageKit',
  internetarchive: 'Internet Archive', internxt: 'Internxt', ionosobj: 'IONOS Object Storage',
  jottacloud: 'Jottacloud', koofr: 'Koofr', linkbox: 'Linkbox', linode: 'Linode Object Storage',
  mailru: 'Mail.ru Cloud', mega: 'MEGA', memory: 'In memory', minio: 'MinIO', netstorage: 'Akamai NetStorage',
  nextcloud: 'Nextcloud', onedrive: 'OneDrive', opencloud: 'OpenCloud', opendrive: 'OpenDrive',
  oracle: 'Oracle Object Storage', ovh: 'OVHcloud Object Storage', owncloud: 'ownCloud', oxicloud: 'OxiCloud',
  pcloud: 'pCloud', pikpak: 'PikPak', pixeldrain: 'Pixeldrain', premiumize: 'premiumize.me',
  protondrive: 'Proton Drive', putio: 'put.io', qingstor: 'QingStor', quatrix: 'Quatrix', r2: 'Cloudflare R2',
  s3: 'S3 compatible', scaleway: 'Scaleway Object Storage', seafile: 'Seafile', seaweedfs: 'SeaweedFS', sftp: 'SFTP',
  shade: 'Shade', sharefile: 'ShareFile', sia: 'Sia', smb: 'SMB / Windows share', spaces: 'DigitalOcean Spaces',
  storj: 'Storj', sugarsync: 'SugarSync', swift: 'OpenStack Swift', synologyc2: 'Synology C2', ulozto: 'Ulož.to',
  union: 'Merged remotes', wasabi: 'Wasabi', webdav: 'WebDAV', yandex: 'Yandex Disk', zoho: 'Zoho WorkDrive',
};

describe('the provider picker', () => {
  const providers = catalog.providers as Provider[];
  const ids = providers.map((p) => p.id);
  const pick = (...want: string[]) => providerPicker(providers.filter((p) => want.includes(p.id)));

  it('finds every provider a saved remote can name, under the name the list has always shown', () => {
    expect(ids).toEqual(expect.arrayContaining(Object.keys(SHOWN)));
    const { items } = providerPicker(providers);
    for (const [id, name] of Object.entries(SHOWN)) {
      expect(remoteKind({ type: 'other', provider: id }, providers), id).toBe(name);
      expect(items.filter((i) => i.id === id), id).toHaveLength(1);
    }
  });

  it('offers the stores people run themselves as object storage, each with what it is on its tooltip', () => {
    const own = ['garage', 'juicefs', 'minio', 'rustfs', 'seaweedfs', 'silo', 'versity'];
    const { items } = pick(...own);
    expect(items.map((i) => [i.label, i.group])).toEqual([
      ['Garage', 'storage'], ['JuiceFS', 'storage'], ['MinIO', 'storage'], ['RustFS', 'storage'],
      ['SeaweedFS', 'storage'], ['Silo', 'storage'], ['Versity', 'storage'],
    ]);
    for (const i of items) expect(i.tip, i.id).toBe('Object storage you run yourself. It needs the address of its endpoint.');
    for (const id of own) expect(markOf(id).mark?.svg, id).toMatch(/^<svg /);
  });

  it('calls Versity by its short name and still finds it by the long one', () => {
    const [versity] = pick('versity').items;
    expect(versity!.label).toBe('Versity');
    expect(versity!.terms).toContain('Versity S3 Gateway');
    expect(providerName('versity')).toBe('Versity');
    expect(suggestName(productName('versity'), [])).toBe('versity');
  });

  it('names a remote by its rclone type while its product is unknown', () => {
    expect(remoteKind({ type: 'webdav', provider: 'nextcloud' }, [])).toBe('webdav');
    expect(remoteKind({ type: 'webdav' }, providers)).toBe('webdav');
  });

  it('stands the providers in the catalog\'s groups, with the remotes built on another one last', () => {
    const { groups, items } = providerPicker(providers);
    expect(groups.map((g) => [g.id, g.title])).toEqual([
      ['cloud', 'Clouds'], ['storage', 'Object storage'], ['protocol', 'Servers and protocols'], ['layered', 'On another remote'],
    ]);
    expect(items.filter((i) => i.group === 'layered').map((i) => i.id).sort())
      .toEqual(['alias', 'archive', 'cache', 'chunker', 'combine', 'compress', 'crypt', 'hasher', 'memory', 'union']);
  });

  it('leaves out what this rclone cannot reach', () => {
    const { items, groups } = pick('smb', 'nextcloud');
    expect(items.map((i) => i.id)).toEqual(['nextcloud', 'smb']);
    expect(groups.map((g) => g.id)).toEqual(['cloud', 'protocol']);
  });

  it('draws a brand with its own logo and colours, and anything else with a glyph this app has', () => {
    const lit = markOf('nextcloud');
    expect(lit.mark?.svg).toMatch(/^<svg /);
    expect(lit.mark?.tile).toBe('#0082c9');
    expect(markOf('smb')).toEqual({ glyph: 'IconFolder' });
    expect(markOf('nothing-like-this')).toEqual({ glyph: 'IconCloud' });
    for (const id of ids) {
      const m = markOf(id);
      if (!m.mark) expect(hasGlyph(m.glyph), id).toBe(true);
    }
  });

  it('says the names that are words in the reader language, with a kind as the second line, and keeps a brand as it is', () => {
    expect(providerName('s3')).toBe('S3 compatible');
    expect(providerName('nextcloud')).toBe('Nextcloud');
    setLanguage('de');
    try {
      expect(providerName('crypt')).toBe('Verschlüsselt');
      const [smb, crypt] = pick('smb', 'crypt').items;
      expect([smb!.label, smb!.sub]).toEqual(['SMB', 'Windows-Freigabe']);
      expect(crypt!.label).toBe('Verschlüsselt');
      expect(crypt!.terms).toContain('Encrypted');
    } finally {
      setLanguage('en');
    }
    expect(nameAndKind('SMB / Windows share')).toEqual({ name: 'SMB', sub: 'Windows share' });
  });

  it('puts a protocol\'s explanation behind its (i) and a cloud\'s on its tooltip', () => {
    const { items } = pick('sftp', 'crypt', 'icloud');
    const by = (id: string) => items.find((i) => i.id === id)!;
    expect(by('sftp').hint).toBeTruthy();
    expect(by('crypt').hint).toBeTruthy();
    expect(by('icloud').tip).toBeTruthy();
    expect(by('icloud').hint).toBeUndefined();
  });

  it('suggests a name from the product, numbered when it is taken', () => {
    expect(suggestName('Nextcloud', [])).toBe('nextcloud');
    expect(suggestName('Nextcloud', ['nextcloud', 'nextcloud-2'])).toBe('nextcloud-3');
    expect(suggestName('Ulož.to', [])).toBe('uloz-to');
    expect(suggestName('Google Drive', [])).toBe('google-drive');
  });
});

describe('the form of a remote', () => {
  const webdav: RemoteType = {
    id: 'webdav', description: 'WebDAV', featured: true, oauth: false, pathHint: 'folder',
    options: [option('url', { required: true }), option('vendor', { essential: true }), option('user', { essential: true }), option('pass', { essential: true, secret: true }), option('bearer_token', { secret: true })],
  };

  it('asks first what the remote needs and leaves out what the product sets itself', () => {
    const { first, rest } = splitOptions(webdav, nextcloud.preset, {});
    expect(first.map((o) => o.name)).toEqual(['url', 'user', 'pass']);
    expect(rest.map((o) => o.name)).toEqual(['vendor', 'bearer_token']);
  });

  it('leaves the token of a cloud that signs in through a browser to the sign-in row', () => {
    const drive: RemoteType = { ...webdav, id: 'drive', oauth: true, options: [option('token', { essential: true }), option('scope', { essential: true })] };
    expect(splitOptions(drive, undefined, {}).first.map((o) => o.name)).toEqual(['scope']);
  });

  it('shows an S3 option only for the providers rclone names it for', () => {
    expect(applies('AWS,Ceph', 'AWS')).toBe(true);
    expect(applies('AWS,Ceph', 'Minio')).toBe(false);
    expect(applies('!AWS,Minio', 'Ceph')).toBe(true);
    expect(applies('!AWS,Minio', 'AWS')).toBe(false);
    expect(applies(undefined, 'AWS')).toBe(true);
    expect(applies('AWS', undefined)).toBe(false);
  });

  it('explains a field in its own words, with the address the product uses and rclone name', () => {
    expect(optionLabel('pass')).toBe('Password');
    expect(optionLabel('use_insecure_cipher')).toBe('use_insecure_cipher');
    const url = optionHint(option('url'), nextcloud, 'webdav')!;
    expect(url).toContain('https://cloud.example.com/remote.php/webdav/');
    expect(url).toContain('rclone calls this field url.');
    const rustfs: Provider = { id: 'rustfs', backend: 's3', preset: { provider: 'Other', description: 'RustFS' }, urlHint: 'http://tower:9000' };
    expect(optionHint(option('endpoint'), rustfs, 's3')).toContain('It looks like this: http://tower:9000');
    expect(optionHint(option('pass'), nextcloud, 'webdav')).toContain('app password goes here');
  });

  it('says how a product signs in where the secret goes', () => {
    const r2: Provider = { id: 'r2', backend: 's3', auth: 'accesskey' };
    expect(optionHint(option('secret_access_key'), r2, 's3')).toContain("service's own console");
    const fichier: Provider = { id: 'fichier', backend: 'fichier', auth: 'apikey', authUrl: 'https://1fichier.com/console/params.pl' };
    expect(optionHint(option('api_key'), fichier, 'fichier')).toContain('You make it at https://1fichier.com/console/params.pl');
  });

  it('falls back to rclone help for a field this app has no words for', () => {
    expect(optionHint(option('chunk_size', { help: 'Upload chunk size.' }), null, 's3')).toBe('Upload chunk size.');
  });
});
