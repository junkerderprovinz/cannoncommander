// Absolute paths as the server sees them. An entry inside an archive carries "::" after the archive's own name.
export const split = (p: string) => p.split('/').filter(Boolean);
export const join = (path: string, name: string) => (path === '/' ? '' : path) + '/' + name;
export function parent(path: string) {
  const p = split(path);
  p.pop();
  return '/' + p.join('/');
}
export const inArchive = (path: string) => path.includes('::');
// The folder that holds the archive path lies in, or path itself outside any archive.
export const outsideArchive = (path: string) => (inArchive(path) ? parent(path.slice(0, path.indexOf('::'))) : path);

const drive = /^\/([A-Za-z])(?=\/|$)/;
// The server shows \\nas\daten as /net/nas/daten.
const net = /^\/net(?=\/|$)/;

// The path as the computer writes it, which on Windows is C:\Users and not the server's /C/Users.
export function nativePath(path: string, windows: boolean) {
  if (windows && net.test(path)) return '\\\\' + path.slice(5).replaceAll('/', '\\');
  if (!windows || !drive.test(path)) return path;
  const p = path.replace(drive, '$1:').replaceAll('/', '\\');
  return p.length === 2 ? p + '\\' : p;
}

// A path typed or pasted in the form the computer writes it, or in the server's, as the server's path. home is the
// server's path of the home folder, which ~ and the variables for it stand for.
export function typedPath(typed: string, { windows, home }: { windows: boolean; home: string }) {
  let p = typed.trim().replace(/^"(.*)"$/, '$1');
  if (/^file:\/\//i.test(p)) {
    try {
      const url = new URL(p);
      p = (windows && url.host ? '//' + url.host : '') + decodeURIComponent(url.pathname).replace(/^\/([A-Za-z]:)/, '$1');
    } catch { /* not a URI after all, so it is taken as typed */ }
  }
  p = p.replace(windows ? /^(~|%USERPROFILE%)(?=[\\/]|$)/i : /^(~|\$HOME)(?=\/|$)/, home);
  if (windows) p = p.replace(/^[\\/]{2}/, '/net/').replace(/^([A-Za-z]):/, (_, d: string) => '/' + d.toUpperCase()).replaceAll('\\', '/');
  return p.length > 1 ? p.replace(/\/+$/, '') : p || '/';
}
