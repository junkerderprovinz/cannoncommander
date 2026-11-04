import { describe, expect, it } from 'vitest';
import { addMessages, setLanguage } from '@/i18n';
import de from './de.json';
import en from './en.json';
import { jobText } from './url';

addMessages('fetch', { de, en });

describe('jobText', () => {
  it('says where the file comes from, as the server names the job', () => {
    setLanguage('de');
    expect(jobText('releases.ubuntu.com/ubuntu.iso')).toBe('Laden: ubuntu.iso von releases.ubuntu.com');
    setLanguage('en');
    expect(jobText('releases.ubuntu.com/ubuntu.iso')).toBe('Load: ubuntu.iso from releases.ubuntu.com');
  });

  it('names only the file for a job stored without its host', () => {
    setLanguage('de');
    expect(jobText('ubuntu.iso')).toBe('Laden: ubuntu.iso');
  });
});
