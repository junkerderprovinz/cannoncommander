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
