// The parts of the path bar after its top, each with the path it opens. A network share mounted deep down, as gvfs
// mounts one below /run/user/1000/gvfs, starts at the network and the share, as Nautilus shows it, rather than at the
// folders it happens to lie in.
import { parent, split } from './paths';

export interface Crumb { path: string; part: string; network?: boolean }

export function crumbs(path: string, share?: string): Crumb[] {
  const parts = split(path);
  const all = parts.map((part, k) => ({ path: '/' + parts.slice(0, k + 1).join('/'), part }));
  if (!share || (path !== share && !path.startsWith(share + '/'))) return all;
  return [{ path: parent(share), part: '', network: true }, ...all.slice(split(share).length - 1)];
}
