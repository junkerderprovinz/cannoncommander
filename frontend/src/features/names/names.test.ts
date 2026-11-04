import { describe, expect, it } from 'vitest';
import { copyName } from './index';

describe('copyName', () => {
  it('puts the word before the ending', () => {
    expect(copyName('notizen.txt', new Set(['notizen.txt']), 'Kopie')).toBe('notizen Kopie.txt');
  });
  it('counts on when the name is taken', () => {
    expect(copyName('notizen.txt', new Set(['notizen Kopie.txt']), 'Kopie')).toBe('notizen Kopie 2.txt');
  });
  it('starts from the original when copying a copy', () => {
    expect(copyName('notizen Kopie 2.txt', new Set(['notizen Kopie.txt', 'notizen Kopie 2.txt']), 'Kopie')).toBe('notizen Kopie 3.txt');
  });
  it('leaves a folder or a hidden file without an ending', () => {
    expect(copyName('Urlaub', new Set(), 'Kopie')).toBe('Urlaub Kopie');
    expect(copyName('.env', new Set(), 'Kopie')).toBe('.env Kopie');
  });
});
