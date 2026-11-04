// Sum lists as sha256sum writes and reads them, and the sizes a file is split into.
import { MB } from '@/core/format';

export const SUMFILE = /\.(md5|sha1|sha256|sha512|b3)$|^(SHA256SUMS|SHA1SUMS|MD5SUMS|SHA512SUMS|B3SUMS)$/i;

// Names are relative to the folder the list is saved in, as sha256sum -c expects them, and full paths where a file
// lies outside it. A name with a backslash or a line break gets the escaping coreutils uses.
export function lineName(hash: string, path: string, dir: string) {
  const name = path.startsWith(dir.replace(/\/$/, '') + '/') ? path.slice(dir.replace(/\/$/, '').length + 1) : path;
  if (/[\\\n\r]/.test(name)) return `\\${hash}  ${name.replace(/\\/g, '\\\\').replace(/\n/g, '\\n').replace(/\r/g, '\\r')}\n`;
  return `${hash}  ${name}\n`;
}

export const partSize = (st: { size: string; custom: string }) => (st.size === 'custom' ? parseFloat(st.custom.replace(',', '.')) || 0 : Number(st.size)) * MB;
