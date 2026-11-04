// The kinds of remote rclone can mount here, and the fields each one asks for. A field's key is rclone's own
// parameter, except the one marked as the path, which names the folder, share or bucket the mount shows.
import { c } from './kit/kit';

export interface RemoteType { id: string; label: string; g: string; sub: string; oauth?: string }
export interface RemoteField { k: string; l: string; ph?: string; def?: string; w?: string; pw?: boolean; opt?: boolean; tip?: string; dd?: [string, string][]; path?: boolean }

export const remoteTypes = (): RemoteType[] => [
  { id: 'sftp', label: 'SFTP', g: 'IconNetwork', sub: c('remotes.sftpSub') },
  { id: 'smb', label: 'SMB', g: 'IconNetwork', sub: c('remotes.smbSub') },
  { id: 'webdav', label: 'WebDAV', g: 'IconBrowser', sub: c('remotes.webdavSub') },
  { id: 's3', label: 'S3', g: 'IconCloud', sub: 'AWS, MinIO, R2' },
  { id: 'drive', label: 'Google Drive', g: 'IconCloud', sub: c('remotes.signIn', { who: 'Google' }), oauth: 'Google' },
  { id: 'onedrive', label: 'OneDrive', g: 'IconCloud', sub: c('remotes.signIn', { who: 'Microsoft' }), oauth: 'Microsoft' },
  { id: 'dropbox', label: 'Dropbox', g: 'IconCloud', sub: c('remotes.signIn', { who: 'Dropbox' }), oauth: 'Dropbox' },
];
// A remote set up outside this page keeps whatever type rclone gave it.
export const typeOf = (id: string): RemoteType => remoteTypes().find((t) => t.id === id) ?? { id, label: id, g: 'IconCloud', sub: '' };

export const remoteFields = (type: RemoteType): RemoteField[] => {
  const user = { k: 'user', l: c('remotes.user') }, pass = { k: 'pass', l: c('remotes.password'), pw: true };
  if (type.oauth) return [{ k: 'folder', l: c('remotes.folder'), ph: '/', opt: true, tip: c('remotes.folderTip'), path: true }];
  const byType: Record<string, RemoteField[]> = {
    sftp: [{ k: 'host', l: c('remotes.host'), ph: 'nas.lan' }, { k: 'port', l: 'Port', def: '22', w: 's' }, user, pass],
    smb: [{ k: 'host', l: c('remotes.host'), ph: 'keller' }, { k: 'share', l: c('remotes.share'), ph: 'archiv', path: true }, user, pass, { k: 'domain', l: c('remotes.domain'), ph: 'WORKGROUP', opt: true }],
    webdav: [{ k: 'url', l: c('remotes.url'), ph: 'https://cloud.example.org/remote.php/dav/files/jdp', w: 'w' }, { k: 'vendor', l: c('remotes.vendor'), dd: [['nextcloud', 'Nextcloud'], ['owncloud', 'ownCloud'], ['other', c('remotes.other')]] }, user, pass],
    s3: [{ k: 'provider', l: c('remotes.vendor'), dd: [['AWS', 'AWS'], ['Minio', 'MinIO'], ['Wasabi', 'Wasabi'], ['Cloudflare', 'Cloudflare R2'], ['Other', c('remotes.other')]] },
      { k: 'endpoint', l: c('remotes.endpoint'), ph: 'https://s3.eu-central-1.amazonaws.com', w: 'w', opt: true, tip: c('remotes.endpointTip') }, { k: 'region', l: 'Region', ph: 'eu-central-1' },
      { k: 'access_key_id', l: 'Access Key' }, { k: 'secret_access_key', l: 'Secret Key', pw: true }, { k: 'bucket', l: 'Bucket', path: true }],
  };
  return byType[type.id] ?? [];
};
