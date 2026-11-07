import { describe, expect, it } from 'vitest';
import { shownTile, TILE_MAX, TILE_MIN, TILE_STEP, thumbSize, wheelZoom, type Zoom, zoomBy, zoomMod } from './zoom';

// One notch at a time from the densest view up, and what each notch shows.
const walk = (from: Zoom, notches: number, n: number) => {
  const seen: Zoom[] = [];
  let at: Zoom = from, rest = 0;
  for (let k = 0; k < n; k++) {
    const got = zoomBy(at, notches + rest);
    rest = got.rest;
    at = { view: got.view, tile: got.tile };
    seen.push(at);
  }
  return seen;
};

describe('the zoom scale', () => {
  it('steps through the row views one notch each', () => {
    expect(walk({ view: 'compact' }, 1, 3).map((z) => z.view)).toEqual(['details', 'list', 'tiles']);
  });

  it('enters the cards at their smallest size and grows them by a step per notch', () => {
    const [tiles, next] = walk({ view: 'list' }, 1, 2);
    expect(tiles).toEqual({ view: 'tiles', tile: TILE_MIN });
    expect(next).toEqual({ view: 'tiles', tile: TILE_MIN + TILE_STEP });
  });

  it('turns tiles into large ones and those into a gallery as they grow', () => {
    const views = walk({ view: 'tiles', tile: TILE_MIN }, 1, 40).map((z) => z.view);
    expect([...new Set(views)]).toEqual(['tiles', 'large', 'gallery']);
  });

  it('grows without steps between notches, as a pinch does', () => {
    const got = zoomBy({ view: 'large', tile: 200 }, 0.25);
    expect(got).toMatchObject({ view: 'large', tile: 200 + TILE_STEP / 4, rest: 0 });
  });

  it('walks back down the same way', () => {
    const up = walk({ view: 'compact' }, 1, 30);
    const down = walk(up.at(-1)!, -1, 30);
    expect(down.at(-1)).toEqual({ view: 'compact', tile: undefined });
    expect(down.map((z) => z.view).filter((v, k, a) => v !== a[k - 1])).toEqual(['gallery', 'large', 'tiles', 'list', 'details', 'compact']);
  });

  it('stops at both ends', () => {
    expect(zoomBy({ view: 'compact' }, -3)).toEqual({ view: 'compact', tile: undefined, rest: 0 });
    expect(zoomBy({ view: 'gallery', tile: TILE_MAX - 4 }, 5)).toEqual({ view: 'gallery', tile: TILE_MAX, rest: 0 });
  });

  it('keeps a part of a notch below the smallest cards until it adds up to one', () => {
    const half = zoomBy({ view: 'tiles', tile: TILE_MIN }, -0.5);
    expect(half).toEqual({ view: 'tiles', tile: TILE_MIN, rest: -0.5 });
    expect(zoomBy({ view: half.view, tile: half.tile }, -0.5 + half.rest).view).toBe('list');
  });

  it('keeps a part of a notch in the row views', () => {
    expect(zoomBy({ view: 'details' }, 0.6)).toEqual({ view: 'details', tile: undefined, rest: 0.6 });
  });

  it('reads a card view without a size of its own as a size inside its range', () => {
    expect(zoomBy({ view: 'large' }, 0).tile).toBeGreaterThanOrEqual(TILE_MIN);
    expect(zoomBy({ view: 'gallery' }, 0)).toMatchObject({ view: 'gallery' });
    expect(zoomBy({ view: 'tiles', tile: 9999 }, 0)).toMatchObject({ view: 'tiles' });
  });

  it('steps from a view off the scale in between the list and the tiles', () => {
    for (const view of ['content', 'columns', 'drives']) {
      expect(zoomBy({ view }, 1)).toMatchObject({ view: 'tiles', tile: TILE_MIN });
      expect(zoomBy({ view }, -1)).toMatchObject({ view: 'list' });
      expect(zoomBy({ view }, 0.4)).toMatchObject({ view, rest: 0.4 });
    }
  });
});

describe('the size a list shows', () => {
  it('is the zoomed size, or what the view is drawn at without one', () => {
    expect([shownTile('large', 200), shownTile('large', undefined), shownTile('tiles', undefined)]).toEqual([200, 208, 144]);
  });

  it('is the smallest card in a row view, so the tiles grow out of the list', () => {
    expect(shownTile('list', undefined)).toBe(TILE_MIN);
  });
});

describe('the thumbnail size', () => {
  it('asks for the small pictures until a card outgrows them', () => {
    expect(thumbSize(undefined, 1)).toBe('small');
    expect(thumbSize(240, 1)).toBe('small');
    expect(thumbSize(300, 1)).toBe('large');
  });

  it('counts the pixels of the screen', () => {
    expect(thumbSize(160, 2)).toBe('large');
  });
});

describe('zooming with the wheel', () => {
  const wheel = (deltaY: number, mods: { ctrlKey?: boolean; metaKey?: boolean } = { ctrlKey: true }, timeStamp = 0, deltaMode = 0) => ({ deltaY, deltaMode, timeStamp, ctrlKey: false, metaKey: false, ...mods });
  const setup = (view: Zoom) => {
    let at = view;
    const zoom = wheelZoom(() => at, (z) => { at = z; });
    return { zoom, at: () => at };
  };

  it('zooms in with ctrl and the wheel turned up', () => {
    const s = setup({ view: 'details' });
    expect(s.zoom(wheel(-100))).toBe(true);
    expect(s.at().view).toBe('list');
  });

  it('zooms with cmd as well', () => {
    const s = setup({ view: 'details' });
    expect(s.zoom(wheel(100, { metaKey: true }))).toBe(true);
    expect(s.at().view).toBe('compact');
  });

  it('leaves a wheel without the key alone', () => {
    const s = setup({ view: 'details' });
    expect(s.zoom(wheel(-100, {}))).toBe(false);
    expect(s.at().view).toBe('details');
  });

  it('adds up the small steps of a trackpad pinch', () => {
    const s = setup({ view: 'details' });
    for (let k = 0; k < 9; k++) s.zoom(wheel(-10, { ctrlKey: true }, k * 16));
    expect(s.at().view).toBe('details');
    s.zoom(wheel(-10, { ctrlKey: true }, 160));
    expect(s.at().view).toBe('list');
  });

  it('pinches the cards smoothly', () => {
    const s = setup({ view: 'large', tile: 200 });
    s.zoom(wheel(-25));
    expect(s.at()).toEqual({ view: 'large', tile: 200 + TILE_STEP / 4 });
  });

  it('forgets a part of a notch after a pause', () => {
    const s = setup({ view: 'details' });
    s.zoom(wheel(-60, { ctrlKey: true }, 0));
    s.zoom(wheel(-60, { ctrlKey: true }, 2000));
    expect(s.at().view).toBe('details');
  });

  it('takes one notch from a wheel that reports a big jump, and reads lines as notches', () => {
    const s = setup({ view: 'compact' });
    s.zoom(wheel(-480));
    expect(s.at().view).toBe('details');
    s.zoom(wheel(-3, { ctrlKey: true }, 1000, 1));
    expect(s.at().view).toBe('list');
  });
});

describe('the key that zooms', () => {
  it('is Cmd on a Mac and Ctrl elsewhere', () => {
    expect(zoomMod('MacIntel')).toBe('Cmd');
    expect(zoomMod('iPad')).toBe('Cmd');
    expect(zoomMod('Win32')).not.toBe('Cmd');
  });
});
