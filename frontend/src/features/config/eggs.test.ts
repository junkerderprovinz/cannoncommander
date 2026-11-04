import { beforeEach, describe, expect, it } from 'vitest';
import { forgetEggs, found, motionPressed, rainbowSwitched, shapePressed } from './eggs';

beforeEach(forgetEggs);

describe('the leaf', () => {
  it('opens on the fifth press of square once square is chosen', () => {
    expect(shapePressed('square', 'round')).toBe('square');
    for (let n = 1; n < 5; n++) expect(shapePressed('square', 'square')).toBe('square');
    expect(shapePressed('square', 'square')).toBe('leaf');
    expect(found.leaf).toBe(true);
  });

  it('starts counting again after a press on another shape', () => {
    for (let n = 0; n < 4; n++) shapePressed('square', 'square');
    shapePressed('round', 'square');
    for (let n = 0; n < 4; n++) expect(shapePressed('square', 'square')).toBe('square');
    expect(found.leaf).toBe(false);
  });
});

describe('the storm', () => {
  it('opens on the fifth press of wild once wild is chosen', () => {
    const got = [1, 2, 3, 4, 5].map(() => motionPressed('wild', 'wild'));
    expect(got).toEqual(['wild', 'wild', 'wild', 'wild', 'storm']);
    expect(found.storm).toBe(true);
  });

  it('never counts presses on a calmer level', () => {
    for (let n = 0; n < 10; n++) expect(motionPressed('off', 'off')).toBe('off');
    expect(found.storm).toBe(false);
  });
});

describe('disco', () => {
  it('opens after five turn-ons of the rainbow, each within three seconds', () => {
    for (let n = 0; n < 5; n++) {
      rainbowSwitched(true, 1000 + n * 2000);
      rainbowSwitched(false, 1500 + n * 2000);
    }
    expect(found.disco).toBe(true);
  });

  it('stays hidden when the turn-ons are far apart', () => {
    for (let n = 0; n < 8; n++) rainbowSwitched(true, n * 3500);
    expect(found.disco).toBe(false);
  });

  it('counts only turn-ons', () => {
    for (let n = 0; n < 8; n++) rainbowSwitched(false, n * 100);
    expect(found.disco).toBe(false);
  });
});
