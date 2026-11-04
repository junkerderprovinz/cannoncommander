import { describe, expect, it } from 'vitest';
import { asFolder } from './basket';
import { newForm, readForm, SLUG, slugFor } from './link';

describe('share links', () => {
  it('rolls addresses the server takes, with the name in them', () => {
    for (let i = 0; i < 50; i++) expect(slugFor('Mietvertrag Ü 2026.pdf')).toMatch(/^[a-z2-9]{4}-mietvertrag-u-2026$/);
    expect(SLUG.test(slugFor('...'))).toBe(true);
  });

  it('turns the form into the request, and stops at what the server would refuse', () => {
    const f = newForm('urlaub', true);
    expect(readForm({ ...f, slug: 'Ab' })).toBeNull();
    expect(readForm({ ...f, pw: true, pass: 'kurz' })).toBeNull();
    const req = readForm({ ...f, slug: 'Urlaub-2026', exp: '0', limit: '5', pw: true, pass: 'geheim123', uploads: true })!;
    expect(req).toEqual({ slug: 'urlaub-2026', expires: null, limit: 5, notify: false, password: 'geheim123', uploads: true });
    expect(readForm({ ...newForm('a.txt', false), uploads: true })).not.toHaveProperty('uploads');
  });

  it('numbers the same name from two folders, as the other side sees them', () => {
    const shown = asFolder([
      { path: '/a/IMG.jpg', name: 'IMG.jpg', dir: false, size: 1 },
      { path: '/b/IMG.jpg', name: 'IMG.jpg', dir: false, size: 1 },
      { path: '/c/Urlaub', name: 'Urlaub', dir: true, size: 1 },
      { path: '/d/Urlaub', name: 'Urlaub', dir: true, size: 1 },
    ]).map((x) => x.shown);
    expect(shown).toEqual(['IMG.jpg', 'IMG (2).jpg', 'Urlaub', 'Urlaub (2)']);
  });
});
