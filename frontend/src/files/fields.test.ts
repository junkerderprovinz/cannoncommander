import { afterAll, describe, expect, it } from 'vitest';
import { setLanguage } from '@/i18n';
import { localQuery, serverQuery, serverSize } from './fields';

afterAll(() => setLanguage('de'));

describe('the search line on its way to the server', () => {
  it('names columns by their ids in any of the languages', async () => {
    await setLanguage('en');
    expect(serverQuery('*.jpg lens:"Fujinon 23" year:>2000')).toBe('*.jpg lens:"Fujinon 23" year:>2000');
    expect(serverQuery('Lens:fuji cameramodel:x100')).toBe('lens:fuji camera:x100');
    expect(serverQuery('Objektiv:fuji breite:>=3840 codec:hevc')).toBe('lens:fuji width:>=3840 vcodec:hevc');
  });

  it('knows width, height and codec by their words in the language and in English and German', async () => {
    await setLanguage('de');
    expect(serverQuery('breite:>=3840 Höhe:2160 codec:hevc geändert:<7d')).toBe('width:>=3840 height:2160 vcodec:hevc modified:<7d');
    await setLanguage('fr');
    expect(serverQuery('width:>=3840 hoehe:2160 Codec:hevc')).toBe('width:>=3840 height:2160 vcodec:hevc');
  });

  it('turns the words a column shows into the codes it keeps', async () => {
    await setLanguage('de');
    expect(serverQuery('ausrichtung:hochformat blitz:"Kein Blitz"')).toBe('orientation:portrait flash:off');
    expect(serverQuery('tonspuren:englisch untertitel:=Unbekannt')).toBe('audiotracks:en subtitles:=und');
    await setLanguage('en');
    expect(serverQuery('orientation:portrait channels:stereo audiotracks:german')).toBe('orientation:portrait channels:stereo audiotracks:de');
    expect(serverQuery('Ausrichtung:Querformat')).toBe('orientation:landscape');
  });

  it('sends types, kinds and yes or no as codes', async () => {
    await setLanguage('de');
    expect(serverQuery('typ:Bild art:Musik verschlüsselt:ja signiert:nein')).toBe('type:img kind:music encrypted:yes signed:no');
    expect(serverQuery('typ:=Ordner art:sonstiges')).toBe('type:=folder kind:other');
    await setLanguage('en');
    expect(serverQuery('type:"Disk image" kind:picture bootable:Yes Typ:Verknüpfung')).toBe('type:disk kind:image bootable:yes type:link');
    expect(serverQuery('type:pdf kind:doc')).toBe('type:pdf kind:doc');
  });

  it('reads dates the way the language writes them, and the German and English way', async () => {
    await setLanguage('de');
    expect(serverQuery('geändert:>01.05.2026 aufgenommen:6.11.2026')).toBe('modified:>2026-05-01 taken:2026-11-06');
    await setLanguage('en');
    expect(serverQuery('modified:<1/5/2026 created:2026-05 taken:<7d')).toBe('modified:<2026-05-01 created:2026-05 taken:<7d');
    await setLanguage('fr');
    expect(serverQuery('modified:06/11/2026 accessed:5.11.2026')).toBe('modified:2026-11-06 accessed:2026-11-05');
    await setLanguage('ja');
    expect(serverQuery('modified:2026/11/6 taken:05.11.2026 created:２０２６/１１/０６')).toBe('modified:2026-11-06 taken:2026-11-05 created:2026-11-06');
  });

  it('turns dates of other calendars into the common one', async () => {
    await setLanguage('th');
    expect(serverQuery('modified:6/11/2569')).toBe('modified:2026-11-06');
    await setLanguage('fa');
    expect(serverQuery('modified:۱۴۰۵/۰۸/۱۵')).toBe('modified:2026-11-06');
  });

  it('reads sizes and numbers with a decimal comma or point', async () => {
    await setLanguage('de');
    expect(serverQuery('größe:>2,5GB größe:<1.5gb breite:>=3.840.000 jahr:>2000')).toBe('size:>2.5GB size:<1.5GB width:>=3840000 year:>2000');
    await setLanguage('en');
    expect(serverQuery('size:>2,5 disksize:<300kb unpacked:>1,234.5MB duration:>1,5h')).toBe('size:>2.5 disksize:<300KB unpacked:>1234.5MB duration:>1.5h');
    await setLanguage('fr');
    expect(serverQuery('size:>2,5Go')).toBe('size:>2.5GB');
  });

  it('leaves a line it has already turned as it is', async () => {
    for (const lang of ['de', 'en', 'fr', 'ja']) {
      await setLanguage(lang);
      const line = 'type:img kind:music width:>=3840 modified:>2026-05-01 size:>1.234GB encrypted:yes vcodec:hevc *.mkv';
      expect(serverQuery(line)).toBe(line);
    }
  });

  it('leaves names it does not know for the server to name', async () => {
    await setLanguage('en');
    expect(serverQuery('farbe:rot xyz:1')).toBe('farbe:rot xyz:1');
  });

  it('writes the sizes of the search window with a point and a unit', async () => {
    await setLanguage('de');
    expect(serverSize('2,5 GB')).toBe('2.5GB');
    expect(serverSize('300')).toBe('300');
    expect(serverSize('')).toBe('');
    expect(serverSize('viel')).toBe('viel');
  });
});

describe('a saved line shown in the interface', () => {
  const line = 'type:img width:>=3840 modified:>2026-05-01 size:>2.5GB encrypted:yes vcodec:hevc audiotracks:de *.mkv';

  it('reads in German and English', async () => {
    await setLanguage('de');
    expect(localQuery(line)).toBe('typ:Bild breite:>=3840 geändert:>01.05.2026 größe:>2,5GB verschlüsselt:Ja codec:hevc tonspuren:Deutsch *.mkv');
    await setLanguage('en');
    expect(localQuery(line)).toBe('type:Image width:>=3840 modified:>01/05/2026 size:>2.5GB encrypted:Yes codec:hevc audiotracks:German *.mkv');
  });

  it('quotes a word with a space', async () => {
    await setLanguage('en');
    expect(localQuery('type:disk')).toBe('type:"Disk image"');
  });

  it('turns back into the same line in every language', async () => {
    for (const lang of ['de', 'en', 'fr', 'ja', 'ko', 'fa', 'th', 'ar']) {
      await setLanguage(lang);
      expect(serverQuery(localQuery(line))).toBe(line);
    }
  });

  it('shows an older German line in the language of the interface', async () => {
    await setLanguage('en');
    expect(localQuery(serverQuery('breite:>=3840 typ:Bild geändert:>01.05.2026'))).toBe('width:>=3840 type:Image modified:>01/05/2026');
  });
});
