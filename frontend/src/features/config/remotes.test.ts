import { beforeAll, describe, expect, it } from 'vitest';
import type { Provider, RemoteType } from '@/api/admin';
import { addMessages, setLanguage } from '@/i18n';
import catalog from '../../../scripts/fixture-remotes.json';
import de from './de.json';
import en from './en.json';
import { applies, loadMarks, markOf, nameAndKind, optionHint, optionLabel, providerName, splitOptions, suggestName } from './remotes';

beforeAll(async () => {
  addMessages('config', { de, en });
  setLanguage('en');
  await loadMarks();
});

const nextcloud: Provider = {
  id: 'nextcloud', name: 'Nextcloud', backend: 'webdav', group: 'cloud', preset: { vendor: 'nextcloud' }, mark: 'IconNextcloud', auth: 'apppassword',
  urlHint: 'https://cloud.example.com/remote.php/webdav/',
};
const option = (name: string, more: Partial<RemoteType['options'][number]> = {}) => ({
  name, help: '', type: 'string', required: false, essential: false, secret: false, advanced: false, exclusive: false, ...more,
});

describe('the provider picker', () => {
  it('draws a brand with its own logo and colours, and anything else with a glyph', () => {
    const lit = markOf('IconNextcloud');
    expect(lit.svg).toMatch(/^<svg /);
    expect(lit.tile).toEqual({ tile: '#0082c9', ink: '#ffffff' });
    expect(markOf('IconFolder')).toEqual({ glyph: 'IconFolder' });
    expect(markOf('IconNothingLikeThis')).toEqual({ glyph: 'IconCloud' });
  });

  it('gives every provider a logo that is nothing but SVG, or a glyph of its own', () => {
    for (const p of catalog.providers) {
      const mark = markOf(p.mark);
      if (!mark.svg) {
        expect(mark.glyph, p.id).toBe(p.mark);
        continue;
      }
      const box = document.createElement('template');
      box.innerHTML = mark.svg;
      const nodes = [...box.content.childNodes];
      expect(nodes.length, p.id).toBeGreaterThan(0);
      expect(nodes.map((n) => n.nodeName.toLowerCase()), p.id).toEqual(nodes.map(() => 'svg'));
      const resting = nodes.filter((n) => !(n as Element).classList.contains('glim-mark-hover'));
      expect(resting.length, p.id).toBe(1);
    }
  });

  it('says the names that are words in the reader language and keeps a brand as it is', () => {
    expect(providerName({ id: 's3', name: 'S3 compatible' })).toBe('S3 compatible');
    expect(providerName(nextcloud)).toBe('Nextcloud');
    setLanguage('de');
    try {
      expect(providerName({ id: 'crypt', name: 'Encrypted' })).toBe('Verschlüsselt');
    } finally {
      setLanguage('en');
    }
    expect(nameAndKind('SMB / Windows share')).toEqual({ name: 'SMB', sub: 'Windows share' });
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
    expect(optionHint(option('pass'), nextcloud, 'webdav')).toContain('app password goes here');
  });

  it('says how a product signs in where the secret goes', () => {
    const r2: Provider = { id: 'r2', name: 'Cloudflare R2', backend: 's3', group: 'storage', auth: 'accesskey' };
    expect(optionHint(option('secret_access_key'), r2, 's3')).toContain("service's own console");
    const fichier: Provider = { id: 'fichier', name: '1Fichier', backend: 'fichier', group: 'cloud', auth: 'apikey', authUrl: 'https://1fichier.com/console/params.pl' };
    expect(optionHint(option('api_key'), fichier, 'fichier')).toContain('You make it at https://1fichier.com/console/params.pl');
  });

  it('falls back to rclone help for a field this app has no words for', () => {
    expect(optionHint(option('chunk_size', { help: 'Upload chunk size.' }), null, 's3')).toBe('Upload chunk size.');
  });
});
