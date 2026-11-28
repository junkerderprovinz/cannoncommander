import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

// The browser hands out navigator.clipboard only in a secure context, and the
// container is mostly opened over plain http on the local network. A copy
// that writes to it directly does nothing there.

const src = join(__dirname, '..');

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return sources(path);
    return /\.(ts|vue)$/.test(entry.name) && !entry.name.endsWith('.test.ts') ? [path] : [];
  });
}

describe('copying to the clipboard', () => {
  it('goes through copyText everywhere, so it works over plain http too', () => {
    const direct = sources(src).filter((path) => /navigator\.clipboard\??\.writeText/.test(readFileSync(path, 'utf8')));
    expect(direct.map((path) => path.slice(src.length + 1))).toEqual([]);
  });
});
