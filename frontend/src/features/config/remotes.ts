// What the add dialog knows beyond rclone: the words beside each field and the fields a type asks for first. The
// products, their groups and logos come from GlimStone's storage catalog; the server says which of them this rclone
// reaches and how each one connects.
import { catalogPicker, type TileMark } from '@glimstone/core';
import type { StorageGroup, StorageProviderId } from '@glimstone/core/storageProviders';
import type { PickerItem, PickGroup } from '@glimstone/vue';
import { admin, type Provider, type RemoteCatalog, type RemoteOption, type RemoteTest, type RemoteType } from '@/api/admin';
import { fmtSize } from '@/core/format';
import { glyphComponent, hasGlyph } from '@/core/glyphs';
import { locale } from '@/i18n';
import { c } from '@/features/config/shared';
import { twice } from '@/core/lazy';

// The logos weigh a hundred kilobytes, so they come along with the first list or window that shows them.
let storage: typeof import('@glimstone/core/storageProviders') | null = null;
export const loadMarks = async () => { storage ??= await twice(() => import('@glimstone/core/storageProviders'))(); };

let catalog: Promise<RemoteCatalog> | null = null;
// rclone's options only change with rclone, so one answer serves every window. A failed one is asked again.
export const loadCatalog = () => (catalog ??= Promise.all([admin.remoteCatalog(), loadMarks()]).then(([got]) => got, (e: unknown) => { catalog = null; throw e; }));

const entryOf = (id: string) => storage?.STORAGE_PROVIDERS.entries.find((e) => e.id === id);
const glyphOf = (name?: string) => (name && hasGlyph(name) ? name : 'IconCloud');

// A brand's own drawing where the catalog has one, otherwise the glyph that stands for the kind of thing.
export function markOf(id: string): { mark?: TileMark; glyph: string } {
  const e = entryOf(id);
  return e?.mark ? { mark: e.mark, glyph: 'IconCloud' } : { glyph: glyphOf(e?.glyph) };
}

// The catalog's own name, in English for a word too, which a new remote's name is made from.
export const productName = (id: string) => entryOf(id)?.name ?? id;

// A name that is a word rather than a product's own comes in the reader's language.
export function providerName(id: string) {
  const e = entryOf(id);
  return !e ? id : e.word ? c(`remotes.p.${id}`) : e.name;
}

// "SMB / Windows share" becomes the tile's name and its second line.
export function nameAndKind(label: string): { name: string; sub?: string } {
  const at = label.indexOf(' / ');
  return at < 0 ? { name: label } : { name: label.slice(0, at), sub: label.slice(at + 3) };
}

// One line about what a product is, for the ones whose name does not say it.
const HINTS: Record<string, string> = {
  smb: 'smb', sftp: 'sftp', webdav: 'webdav', ftp: 'ftp', http: 'http', crypt: 'crypt', s3: 's3', hidrive: 'hidrive', icloud: 'icloud',
  googlephotos: 'googlephotos', minio: 'selfHosted', seaweedfs: 'selfHosted', ceph: 'selfHosted', garage: 'selfHosted', silo: 'selfHosted',
  rustfs: 'selfHosted', juicefs: 'selfHosted', versity: 'selfHosted', doi: 'doi',
  alias: 'alias', archive: 'archive', cache: 'cache', chunker: 'chunker', combine: 'combine', compress: 'compress', hasher: 'hasher',
  union: 'union', memory: 'memory',
};
export const providerHint = (id: string) => (HINTS[id] ? c(`remotes.ph.${HINTS[id]}`) : undefined);

// The tiles of the products this rclone reaches, found by rclone's name for the backend too, which is what a guide
// online uses. A protocol or a remote built on another one needs its sentence behind an (i), a product's on its
// tooltip.
export function providerPicker(offered: readonly Pick<Provider, 'id' | 'backend'>[]): { items: PickerItem[]; groups: PickGroup[] } {
  if (!storage) return { items: [], groups: [] };
  const { STORAGE_PROVIDERS } = storage;
  const backends = new Map(offered.map((p) => [p.id, p.backend]));
  const titles = Object.fromEntries(STORAGE_PROVIDERS.groups.map((g) => [g, c(`remotes.g.${g}`)])) as Record<StorageGroup, string>;
  return catalogPicker(STORAGE_PROVIDERS, {
    ids: [...backends.keys()] as StorageProviderId[], titles, lang: locale(),
    glyph: (e) => glyphComponent(glyphOf(e.glyph)),
    own: (e) => {
      const { name, sub } = e.word ? nameAndKind(c(`remotes.p.${e.id}`)) : { name: e.name, sub: undefined };
      const why = providerHint(e.id), behind = e.group === 'protocol' || e.group === 'layered';
      return { label: name, sub, terms: [backends.get(e.id)!], hint: behind ? why : undefined, tip: behind ? undefined : why };
    },
  });
}

// Names of the options the forms show most, in the reader's language. Any other keeps rclone's name, which is what
// rclone's documentation searches for.
const LABELS: Record<string, string> = {
  user: 'user', username: 'user', pass: 'pass', password: 'pass', host: 'host', url: 'url', port: 'port', domain: 'domain', vendor: 'vendor',
  provider: 'provider', region: 'region', endpoint: 'endpoint', account: 'account', key: 'key', key_file: 'keyFile', access_key_id: 'accessKey',
  secret_access_key: 'secretKey', access_grant: 'accessGrant', client_id: 'clientId', client_secret: 'clientSecret', remote: 'remote',
  library: 'library', hostname: 'hostname', scope: 'scope', '2fa': 'twoFactor', share_name: 'shareName', upstreams: 'upstreams',
  access_token: 'accessToken', api_key: 'apiKey', api_url: 'apiUrl', api_password: 'apiPassword', passphrase: 'passphrase', tls: 'tls',
  explicit_tls: 'explicitTls', host_keys: 'hostKeys',
};
export const optionLabel = (name: string) => (LABELS[name] ? c(`remotes.o.${LABELS[name]}`) : name);

// What goes into a field, in this app's words, for the common ones.
const OWN_HINTS: Record<string, string> = {
  url: 'url', user: 'user', username: 'user', pass: 'pass', password: 'pass', host: 'host', port: 'port', access_key_id: 'accessKey',
  secret_access_key: 'secretKey', endpoint: 'endpoint', region: 'region', vendor: 'vendor', key_file: 'keyFile', remote: 'remote',
  upstreams: 'upstreams', passphrase: 'passphrase', host_keys: 'hostKeys',
};
const SECRET_FIELDS = new Set(['pass', 'password', 'secret_access_key', 'key', 'api_key', 'token', 'access_token', 'private_key']);
const AUTH: Record<string, string> = {
  apppassword: 'authAppPassword', accesskey: 'authAccessKey', apikey: 'authApiKey', login: 'authLogin', logintoken: 'authLoginToken',
};

// Everything the bubble beside one field says: this app's sentence, how the product signs in, the shape of its address
// and what its server assumes, and rclone's name for the field. rclone's own help only stands in for a field this app
// has nothing to say about.
export function optionHint(o: Pick<RemoteOption, 'name' | 'help'>, provider: Provider | null, type: string): string | undefined {
  let own = OWN_HINTS[o.name] ? c(`remotes.oh.${OWN_HINTS[o.name]}`) : undefined;
  if ((o.name === 'pass' || o.name === 'password') && type === 'webdav') own = c('remotes.oh.passCloud');
  const about: string[] = [];
  if ((o.name === 'url' || o.name === 'endpoint') && provider?.urlHint) about.push(c('remotes.oh.shape', { shape: provider.urlHint }));
  const assumed = provider?.defaults?.[o.name];
  if (assumed) about.push(c('remotes.oh.serverDefault', { value: assumed }));
  if (SECRET_FIELDS.has(o.name) && provider?.auth && AUTH[provider.auth] && type !== 'webdav') {
    about.push(c(`remotes.oh.${AUTH[provider.auth]}`));
    own = undefined;
  }
  if (SECRET_FIELDS.has(o.name) && provider?.authUrl) about.push(c('remotes.oh.authWhere', { url: provider.authUrl }));
  const named = LABELS[o.name] ? c('remotes.oh.rcloneName', { name: o.name }) : undefined;
  if (own || about.length || named) return [own, ...about, named].filter(Boolean).join(' ');
  return o.help || undefined;
}

// rclone's provider filter on an option or an example: a comma list, or one starting with ! that names the exceptions.
export function applies(filter: string | undefined, value: string | undefined) {
  if (!filter) return true;
  const not = filter.startsWith('!');
  const hit = !!value && (not ? filter.slice(1) : filter).split(',').includes(value);
  return not ? !hit : hit;
}

// The fields a form asks first: what rclone needs and what a remote of the type will not work without, but not what
// the product sets by itself, nor the OAuth token, which the sign-in row brings.
export function splitOptions(type: RemoteType, preset: Record<string, string> | undefined, values: Record<string, string>) {
  const shown = type.options.filter((o) => applies(o.provider, values.provider) && !(type.oauth && o.name === 'token'));
  const first = shown.filter((o) => (o.required || o.essential) && preset?.[o.name] === undefined);
  return { first, rest: shown.filter((o) => !first.includes(o)) };
}

// A name from the product's own, numbered when it is taken: nextcloud, nextcloud-2.
export function suggestName(product: string, taken: string[]) {
  const base = product.toLowerCase().normalize('NFD').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'remote';
  let name = base;
  for (let n = 2; taken.includes(name); n++) name = `${base}-${n}`;
  return name;
}

// What a list calls a saved remote: its product, or rclone's type where the parameters name none.
export const remoteKind = (r: { type: string; provider?: string }, providers: Provider[]) =>
  (r.provider && providers.some((x) => x.id === r.provider) ? providerName(r.provider) : r.type);

// What a test found, in the reader's words. path is the folder the remote opens in.
export function testVerdict(x: RemoteTest, path: string) {
  if (x.ok) return x.free != null ? c('remotes.okFree', { free: fmtSize(x.free) }) : c('remotes.ok');
  if (x.failure === 'missing') return c('remotes.fail.missing', { path });
  if (x.failure) return c(`remotes.fail.${x.failure}`);
  return c('remotes.fail.other', { error: x.error ?? '' });
}

// The questions rclone's setups ask, by the parameter that answers them.
const QUESTIONS: Record<string, string> = { config_driveid: 'drive', config_workspace: 'workspace', config_team_drive_id: 'team' };
export const questionLabel = (name: string) => c(`remotes.q.${QUESTIONS[name] ?? 'other'}`);
