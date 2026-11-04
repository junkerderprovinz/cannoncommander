// What a visitor of a share link talks to: /api/v1/s/{slug}. After the password the server sets a cookie for the link,
// so plain links to files work too. The token also goes along in X-Share-Token for the requests made here.
import { BASE, call, json } from './client';

export interface PublicShare {
  slug: string; locked: boolean; kind?: 'file' | 'folder' | 'collection'; name?: string; message?: string; expires?: string;
  uploads?: boolean; uploadRoom?: number; size?: number; mtime?: string; count?: number;
}
export interface PublicItem { name: string; dir: boolean; size: number; mtime: string; format?: string }
export interface PublicListing { path: string; name: string; items: PublicItem[] }
export interface ShareUpload { id: string; path: string; size: number; offset: number; state: 'open' | 'done' | 'skipped'; chunk: number }

const q = (params: Record<string, string>) => '?' + new URLSearchParams(params).toString();
export const inside = (dir: string, name: string) => (dir === '/' ? '/' : dir + '/') + name;

export function shareApi(slug: string) {
  const at = '/s/' + encodeURIComponent(slug);
  let token = '';
  const headers = () => (token ? { 'X-Share-Token': token } : undefined);
  const get = async <T>(path: string) => (await call<T>(at + path, { headers: headers() })).body!;
  return {
    open: () => get<PublicShare>(''),
    async unlock(password: string) {
      token = (await call<{ token: string }>(at + '/unlock', json('POST', { password }))).body!.token;
    },
    list: (path: string) => get<PublicListing>('/list' + q({ path })),
    // Viewing in the browser is free, a download counts against the limit of the link.
    rawUrl: (path: string, download = false) => BASE + at + '/raw' + q({ path, ...(download ? { download: '1' } : {}) }),
    zipUrl: (path = '/') => BASE + at + '/raw' + q({ path, download: '1', format: 'zip' }),
    async text(path: string, max = 256 * 1024): Promise<string> {
      const res = await fetch(BASE + at + '/raw' + q({ path }), { headers: { Range: `bytes=0-${max - 1}`, ...headers() } });
      if (!res.ok) throw new Error(String(res.status));
      return res.text();
    },
    // Sends a file in the chunks the server asks for. onProgress gets the bytes that arrived so far.
    async upload(dir: string, file: File, onProgress: (sent: number) => void, signal?: AbortSignal): Promise<ShareUpload> {
      const name = file.webkitRelativePath || file.name;
      let up = (await call<ShareUpload>(at + '/upload', { ...json('POST', { path: dir, name, size: file.size, mtime: new Date(file.lastModified).toISOString() }), headers: { 'Content-Type': 'application/json', ...headers() }, signal })).body!;
      while (up.state === 'open') {
        const chunk = file.slice(up.offset, up.offset + (up.chunk || 8 << 20));
        up = (await call<ShareUpload>(`${at}/upload/${encodeURIComponent(up.id)}`, {
          method: 'PUT', body: chunk, signal,
          headers: { 'Content-Type': 'application/octet-stream', 'Upload-Offset': String(up.offset), ...headers() },
        })).body!;
        onProgress(up.offset);
      }
      return up;
    },
  };
}
export type ShareApi = ReturnType<typeof shareApi>;
