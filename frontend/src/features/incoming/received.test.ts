import { describe, expect, it } from 'vitest';
import { noteFile, noteName } from './received';

describe('a shared link or text', () => {
  it('is named after its title, without the characters a file name cannot carry', () => {
    expect(noteName('Rezept: Kaiserschmarrn', 'Geteilt')).toBe('Rezept Kaiserschmarrn.txt');
    expect(noteName('a/b\\c?', 'Geteilt')).toBe('a b c.txt');
    expect(noteName('   ', 'Geteilt')).toBe('Geteilt.txt');
    expect(noteName(undefined, 'Geteilt')).toBe('Geteilt.txt');
    expect(noteName('x'.repeat(200), 'Geteilt')).toHaveLength(84);
  });

  it('keeps title, text and address one per line', async () => {
    const f = noteFile({ titel: 'Kaiserschmarrn', adresse: 'https://example.org/k' }, 'Geteilt', 1000);
    expect(f.name).toBe('Kaiserschmarrn.txt');
    expect(f.lastModified).toBe(1000);
    expect(await f.text()).toBe('Kaiserschmarrn\nhttps://example.org/k\n');
  });
});
