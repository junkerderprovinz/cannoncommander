import { COLOR_NAMES_DE, nearestColorName } from '@glimstone/core';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { addMessages, setLanguage } from '@/i18n';
import de from '@/features/labels/de.json';
import en from '@/features/labels/en.json';
import { useLabels } from './labels';

addMessages('labels', { de, en });

function answer(byPath: Record<string, unknown>) {
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    const path = new URL(url, 'http://x').pathname.replace('/api/v1', '') + new URL(url, 'http://x').search;
    const body = byPath[path] ?? byPath[path.split('?')[0]!];
    return new Response(JSON.stringify(body ?? {}), { status: 200, headers: { 'Content-Type': 'application/json' } });
  }));
}
const settle = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => {
  setActivePinia(createPinia());
  setLanguage('de');
  answer({
    '/labels': { colours: [{ id: 'red', name: '', hex: '#ff6b72', count: 1 }, { id: 'c1', name: 'Petrol', hex: '#1b6b73', count: 0 }], tags: [{ name: 'wichtig', count: 1 }] },
    '/labels/items?path=%2Fmnt%2Fuser%2Fdokumente': { path: '/mnt/user/dokumente', items: { 'Mietvertrag.pdf': { colour: 'red', tags: ['wichtig'] } } },
  });
});

describe('colour names', () => {
  it('name the colours the app comes with in the language of the interface', async () => {
    const labels = useLabels();
    await labels.load();
    expect(labels.colourName(labels.colours[0]!)).toBe('Rot');
    setLanguage('en');
    expect(labels.colourName(labels.colours[0]!)).toBe('Red');
    expect(labels.colourName(labels.colours[1]!)).toBe('Petrol');
  });

  it('propose the German name that looks closest', () => {
    expect(nearestColorName('#1b6b73', COLOR_NAMES_DE)).toBe('Petrol');
    expect(nearestColorName('#2ec4b6', COLOR_NAMES_DE)).toBe('Türkis');
    expect(nearestColorName('#ff6b72', COLOR_NAMES_DE)).toBe('Koralle');
  });
});

describe('a search word with #', () => {
  it('finds a file by its tag or by the name of its colour', async () => {
    const labels = useLabels();
    await labels.load();
    const k = { name: 'Mietvertrag.pdf' }, folder = '/mnt/user/dokumente';
    labels.inFolder(folder);
    await settle();
    await settle();
    expect(labels.matches(k, '#wichtig', folder)).toBe(true);
    expect(labels.matches(k, '#Wichtig', folder)).toBe(true);
    expect(labels.matches(k, '#rot', folder)).toBe(true);
    expect(labels.matches(k, '#blau', folder)).toBe(false);
    expect(labels.matches({ name: 'other.pdf' }, '#rot', folder)).toBe(false);
  });
});
