import { afterAll, describe, expect, it } from 'vitest';
import type { Job } from '@/api/ops';
import { addMessages, setLanguage } from '@/i18n';
import jobsDe from '@/features/jobs/de.json';
import jobsEn from '@/features/jobs/en.json';
import de from './de.json';
import en from './en.json';
import { doneText } from './index';

addMessages('transfer', { de, en });
addMessages('jobs', { de: jobsDe, en: jobsEn });
afterAll(() => setLanguage('de'));

const job = (doneFiles: number, same?: number) => ({ doneFiles, result: same === undefined ? undefined : { same, samePaths: [] } }) as unknown as Job;

describe('the toast of a finished copy or move', () => {
  it('says what went over and how many were the same file', async () => {
    await setLanguage('de');
    expect(doneText(job(3), 'copy', 'Urlaub', 'backups')).toBe('Kopiert: Urlaub nach backups');
    expect(doneText(job(3, 2), 'copy', 'Urlaub', 'backups')).toBe('Kopiert: 3 Dateien nach backups. 2 Dateien übersprungen, weil sie am Ziel dieselben Dateien sind');
    expect(doneText(job(0, 1), 'move', 'a.jpg', 'backups')).toBe('Nichts verschoben. Die Datei am Ziel ist dieselbe und bleibt, wie sie ist');
    await setLanguage('en');
    expect(doneText(job(1, 1), 'move', 'Urlaub', 'backups')).toBe('Moved: 1 file to backups. 1 file skipped because it is the same file at the target');
  });
});
