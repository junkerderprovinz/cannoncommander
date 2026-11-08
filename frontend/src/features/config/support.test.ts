import { describe, expect, it } from 'vitest';
import { REPO, versionLink } from './support';

describe('the link on the version number', () => {
  it('opens the notes of a release, with or without a v in the tag', () => {
    expect(versionLink({ version: 'v1.4.0', commit: 'abc1234', release: true })).toBe(`${REPO}/releases/tag/v1.4.0`);
    expect(versionLink({ version: '1.4.0', commit: 'abc1234', release: true })).toBe(`${REPO}/releases/tag/v1.4.0`);
  });

  it('opens the commit a build between releases was made from', () => {
    expect(versionLink({ version: 'dev-abc1234', commit: 'abc1234def', release: false })).toBe(`${REPO}/commit/abc1234def`);
    expect(versionLink({ version: 'v1.4.0-3-gabc1234', commit: 'abc1234def', release: false })).toBe(`${REPO}/commit/abc1234def`);
  });

  it('falls back to the repository when the commit is not known', () => {
    expect(versionLink({ version: 'dev', commit: 'unknown', release: false })).toBe(REPO);
  });
});
