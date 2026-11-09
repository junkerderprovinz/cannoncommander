// What the add dialog knows beyond rclone: products by name with their logos, the words beside each field, and the
// fields a type asks for first. The server sends the product table and rclone's options; the sentences live here.
import { admin, type Provider, type ProviderGroup, type RemoteCatalog, type RemoteOption, type RemoteType } from '@/api/admin';
import { hasGlyph } from '@/core/glyphs';
import { c } from '@/features/config/shared';

// The logos weigh a hundred kilobytes, so they come along with the first list or window that shows them.
let marks: typeof import('./brandMarks') | null = null;
export const loadMarks = async () => { marks ??= await import('./brandMarks'); };

let catalog: Promise<RemoteCatalog> | null = null;
// rclone's options only change with rclone, so one answer serves every window. A failed one is asked again.
export const loadCatalog = () => (catalog ??= Promise.all([admin.remoteCatalog(), loadMarks()]).then(([got]) => got, (e: unknown) => { catalog = null; throw e; }));

export const GROUPS: ProviderGroup[] = ['cloud', 'storage', 'protocol'];

// A brand's own drawing where there is one, otherwise a glyph that stands for the kind of thing.
export function markOf(mark?: string): { svg?: string; glyph: string; tile?: { tile: string; ink: string } } {
  const svg = mark && marks?.BRAND_MARKS[mark];
  if (svg) return { svg, glyph: 'IconCloud', tile: marks!.BRAND_TILES[mark!] };
  return { glyph: mark && hasGlyph(mark) ? mark : 'IconCloud' };
}

// The few names that are words rather than a product's own, which a language says its own way.
const WORD_NAMES = new Set(['smb', 's3', 'crypt', 'alias', 'archive', 'cache', 'chunker', 'combine', 'compress', 'hasher', 'union', 'memory']);
export const providerName = (p: { id: string; name: string }) => (WORD_NAMES.has(p.id) ? c(`remotes.p.${p.id}`) : p.name);

// "SMB / Windows share" becomes the tile's name and its second line.
export function nameAndKind(label: string): { name: string; sub?: string } {
  const at = label.indexOf(' / ');
  return at < 0 ? { name: label } : { name: label.slice(0, at), sub: label.slice(at + 3) };
}

// One line about what a product is, for the ones whose name does not say it.
const HINTS: Record<string, string> = {
  smb: 'smb', sftp: 'sftp', webdav: 'webdav', ftp: 'ftp', http: 'http', crypt: 'crypt', s3: 's3', hidrive: 'hidrive', icloud: 'icloud',
  googlephotos: 'googlephotos', minio: 'selfHosted', seaweedfs: 'selfHosted', ceph: 'selfHosted', garage: 'selfHosted', doi: 'doi',
  alias: 'alias', archive: 'archive', cache: 'cache', chunker: 'chunker', combine: 'combine', compress: 'compress', hasher: 'hasher',
  union: 'union', memory: 'memory',
};
export const providerHint = (id: string) => (HINTS[id] ? c(`remotes.ph.${HINTS[id]}`) : undefined);

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
  if (o.name === 'url' && provider?.urlHint) about.push(c('remotes.oh.shape', { shape: provider.urlHint }));
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
export const remoteKind = (r: { type: string; provider?: string }, providers: Provider[]) => {
  const p = providers.find((x) => x.id === r.provider);
  return p ? providerName(p) : r.type;
};

// The questions rclone's setups ask, by the parameter that answers them.
const QUESTIONS: Record<string, string> = { config_driveid: 'drive', config_workspace: 'workspace', config_team_drive_id: 'team' };
export const questionLabel = (name: string) => c(`remotes.q.${QUESTIONS[name] ?? 'other'}`);
