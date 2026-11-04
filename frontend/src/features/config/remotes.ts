// The kinds of remote rclone can mount here, and the fields each one asks for.
import { c } from './kit/kit';

export interface RemoteType { id: string; label: string; g: string; sub: string; oauth?: string; rclone?: string }
export interface RemoteField { k: string; l: string; ph?: string; def?: string; w?: string; pw?: boolean; opt?: boolean; tip?: string; dd?: [string, string][] }

export const remoteTypes = (): RemoteType[] => [
  { id: 'sftp', label: 'SFTP', g: 'IconNetwork', sub: c('remotes.sftpSub') },
  { id: 'smb', label: 'SMB', g: 'IconNetwork', sub: c('remotes.smbSub') },
  { id: 'webdav', label: 'WebDAV', g: 'IconBrowser', sub: c('remotes.webdavSub') },
  { id: 's3', label: 'S3', g: 'IconCloud', sub: 'AWS, MinIO, R2' },
  { id: 'gdrive', label: 'Google Drive', g: 'IconCloud', sub: c('remotes.signIn', { who: 'Google' }), oauth: 'Google', rclone: 'drive' },
  { id: 'onedrive', label: 'OneDrive', g: 'IconCloud', sub: c('remotes.signIn', { who: 'Microsoft' }), oauth: 'Microsoft', rclone: 'onedrive' },
  { id: 'dropbox', label: 'Dropbox', g: 'IconCloud', sub: c('remotes.signIn', { who: 'Dropbox' }), oauth: 'Dropbox', rclone: 'dropbox' },
];
export const typeOf = (id: string) => remoteTypes().find((t) => t.id === id || t.label === id) ?? remoteTypes()[0]!;

export const remoteFields = (type: RemoteType): RemoteField[] => {
  const user = { k: 'user', l: c('remotes.user') }, pass = { k: 'pass', l: c('remotes.password'), pw: true };
  if (type.oauth) return [{ k: 'folder', l: c('remotes.folder'), ph: '/', opt: true, tip: c('remotes.folderTip') }];
  return {
    sftp: [{ k: 'host', l: c('remotes.host'), ph: 'nas.lan' }, { k: 'port', l: 'Port', def: '22', w: 's' }, user, pass],
    smb: [{ k: 'host', l: c('remotes.host'), ph: 'keller' }, { k: 'share', l: c('remotes.share'), ph: 'archiv' }, user, pass, { k: 'domain', l: c('remotes.domain'), ph: 'WORKGROUP', opt: true }],
    webdav: [{ k: 'url', l: c('remotes.url'), ph: 'https://cloud.example.org/remote.php/dav/files/jdp', w: 'w' }, { k: 'vendor', l: c('remotes.vendor'), dd: [['nextcloud', 'Nextcloud'], ['owncloud', 'ownCloud'], ['other', c('remotes.other')]] }, user, pass],
    s3: [{ k: 'provider', l: c('remotes.vendor'), dd: [['aws', 'AWS'], ['minio', 'MinIO'], ['wasabi', 'Wasabi'], ['r2', 'Cloudflare R2'], ['other', c('remotes.other')]] },
      { k: 'endpoint', l: c('remotes.endpoint'), ph: 'https://s3.eu-central-1.amazonaws.com', w: 'w', opt: true, tip: c('remotes.endpointTip') }, { k: 'region', l: 'Region', ph: 'eu-central-1' },
      { k: 'key', l: 'Access Key' }, { k: 'secret', l: 'Secret Key', pw: true }, { k: 'bucket', l: 'Bucket' }],
  }[type.id] as RemoteField[];
};
