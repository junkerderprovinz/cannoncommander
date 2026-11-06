import { afterAll, describe, expect, it } from 'vitest';
import { setLanguage } from '@/i18n';
import { serverQuery } from './fields';

afterAll(() => setLanguage('de'));

describe('the search line on its way to the server', () => {
  it('names columns by their ids in any of the languages', async () => {
    await setLanguage('en');
    expect(serverQuery('*.jpg lens:"Fujinon 23" year:>2000')).toBe('*.jpg lens:"Fujinon 23" year:>2000');
    expect(serverQuery('Lens:fuji cameramodel:x100')).toBe('lens:fuji camera:x100');
    expect(serverQuery('Objektiv:fuji breite:>=3840 codec:hevc')).toBe('lens:fuji breite:>=3840 codec:hevc');
  });

  it('turns the words a column shows into the codes it keeps', async () => {
    await setLanguage('de');
    expect(serverQuery('ausrichtung:hochformat blitz:"Kein Blitz"')).toBe('orientation:portrait flash:off');
    expect(serverQuery('tonspuren:englisch untertitel:=Unbekannt')).toBe('audiotracks:en subtitles:=und');
    await setLanguage('en');
    expect(serverQuery('orientation:portrait channels:stereo audiotracks:german')).toBe('orientation:portrait channels:stereo audiotracks:de');
    expect(serverQuery('Ausrichtung:Querformat')).toBe('orientation:landscape');
  });

  it('leaves names it does not know for the server to name', async () => {
    await setLanguage('en');
    expect(serverQuery('farbe:rot xyz:1')).toBe('farbe:rot xyz:1');
  });
});
