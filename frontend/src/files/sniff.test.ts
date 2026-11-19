import { afterEach, describe, expect, it, vi } from 'vitest';
import { nameTellsNothing, textInside } from './sniff';

const answer = (status: number) => vi.fn(async () => new Response(status === 200 ? '{"text":""}' : '{}', { status, headers: { 'Content-Type': 'application/json' } }));
afterEach(() => vi.unstubAllGlobals());

describe('a name that tells nothing', () => {
  it('has no extension, or one no format claims', () => {
    expect(['nfo', 'README', 'notizen.xyz', 'Fotos 2026.06'].filter((name) => nameTellsNothing({ name }))).toHaveLength(4);
  });

  it('is not one with a known extension, nor a folder', () => {
    expect(['film.nfo', 'brief.txt', 'bild.jpg', 'syslog.1', 'film.mkv'].some((name) => nameTellsNothing({ name }))).toBe(false);
    expect(nameTellsNothing({ name: 'nfo', dir: true })).toBe(false);
  });
});

describe('looking inside', () => {
  it('finds text where the server reads the start of the file as text', async () => {
    const fetch = answer(200);
    vi.stubGlobal('fetch', fetch);
    expect(await textInside('/mnt/user/medien/filme/Citizen Vigilante/nfo', { size: 59, time: 1 })).toBe(true);
    const asked = new URL(String((fetch.mock.calls[0] as unknown[])[0]), 'http://x');
    expect(asked.pathname).toBe('/api/v1/fs/text');
    expect(asked.searchParams.get('path')).toBe('/mnt/user/medien/filme/Citizen Vigilante/nfo');
    expect(Number(asked.searchParams.get('max'))).toBeLessThanOrEqual(8192);
  });

  it('finds none where the server says the file is not text', async () => {
    vi.stubGlobal('fetch', answer(415));
    expect(await textInside('/mnt/user/downloads/objekt', { size: 16, time: 1 })).toBe(false);
  });

  it('asks once for a file until it changes', async () => {
    const fetch = answer(200);
    vi.stubGlobal('fetch', fetch);
    await textInside('/mnt/user/a/nfo', { size: 59, time: 1 });
    await textInside('/mnt/user/a/nfo', { size: 59, time: 1 });
    expect(fetch).toHaveBeenCalledTimes(1);
    await textInside('/mnt/user/a/nfo', { size: 61, time: 2 });
    expect(fetch).toHaveBeenCalledTimes(2);
  });
});
