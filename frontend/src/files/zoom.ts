// Ctrl and the wheel walk one scale as in Explorer: the row views a notch each, then cards that grow without steps
// and turn into large ones and into a gallery on the way. Content, columns and drives are ways of looking rather
// than sizes, so they sit off the scale and a notch from them steps in between the list and the tiles.
import { t } from '@/i18n';

export interface Zoom { view: string; tile?: number }

export const ROW_STEPS = ['compact', 'details', 'list'];
// Where each card view begins, as the width of one card in pixels.
export const CARD_STEPS: [view: string, from: number][] = [['tiles', 96], ['large', 176], ['gallery', 240]];
export const TILE_MIN = CARD_STEPS[0]![1];
export const TILE_MAX = 400;
export const TILE_STEP = 16;
// What the card views are sized at before anyone zoomed them: 9rem, 13rem and 9rem.
const PLAIN_TILE: Record<string, number> = { tiles: 144, large: 208, gallery: 144 };
// A pinch adds up tenths of a notch, and ten of them must make a whole one.
const EPS = 1e-9;

const cardOf = (tile: number) => CARD_STEPS.findLast(([, from]) => tile >= from)![0];

function tileIn(view: string, tile: number | undefined) {
  const k = CARD_STEPS.findIndex(([v]) => v === view);
  if (k < 0) return undefined;
  const from = CARD_STEPS[k]![1], to = CARD_STEPS[k + 1] ? CARD_STEPS[k + 1]![1] - 1 : TILE_MAX;
  return Math.min(Math.max(tile ?? PLAIN_TILE[view]!, from), to);
}

// Moves by a number of notches, upwards for bigger. What does not make a whole step of a row view yet comes back
// as rest, for the next turn of the wheel to add to.
export function zoomBy(at: Zoom, notches: number): Zoom & { rest: number } {
  const last = ROW_STEPS.length - 1;
  let row = ROW_STEPS.indexOf(at.view), tile = tileIn(at.view, at.tile), acc = notches;
  for (;;) {
    if (tile !== undefined) {
      tile = Math.min(tile + acc * TILE_STEP, TILE_MAX);
      acc = 0;
      if (tile < TILE_MIN) {
        acc = (tile - TILE_MIN) / TILE_STEP;
        tile = TILE_MIN;
        if (acc <= -1 + EPS) {
          [tile, row, acc] = [undefined, last, acc + 1];
          continue;
        }
      }
      return { view: cardOf(tile), tile, rest: acc };
    }
    if (acc >= 1 - EPS) {
      acc -= 1;
      if (row === last || row < 0) tile = TILE_MIN;
      else row++;
    } else if (acc <= -1 + EPS) {
      if (row === 0) return { view: ROW_STEPS[0]!, tile: undefined, rest: 0 };
      acc += 1;
      row = row < 0 ? last : row - 1;
    } else return row < 0 ? { ...at, rest: acc } : { view: ROW_STEPS[row]!, tile: undefined, rest: acc };
  }
}

// What a list is sized at, so the first notch grows cards from where they stood and a list turns into the smallest.
export const shownTile = (view: string, tile: number | undefined) => tile ?? PLAIN_TILE[view] ?? TILE_MIN;

// The server keeps every picture at 256 and at 640 pixels, and a card gets the next one up from what it shows.
export const thumbSize = (tile: number | undefined, dpr: number) => (tile && tile * dpr > 256 ? 'large' : 'small');

// A Mac zooms with Cmd, as its own apps do.
export const zoomMod = (platform = navigator.platform) => (/^(Mac|iP)/.test(platform) ? 'Cmd' : t('keys.mod.ctrl'));

export interface WheelLike { deltaY: number; deltaMode: number; ctrlKey: boolean; metaKey: boolean; timeStamp: number }
const NOTCH = 100;
const PAUSE = 500;

// A mouse sends whole notches, a trackpad pinch many small steps with Ctrl held. A wheel that reports a big jump
// still counts one notch, so a fast spin walks the scale instead of leaping to its end.
export function wheelZoom(get: () => Zoom, set: (z: Zoom) => void) {
  let rest = 0, last = -Infinity;
  return (e: WheelLike) => {
    if (!e.ctrlKey && !e.metaKey) return false;
    const px = e.deltaY * (e.deltaMode === 1 ? NOTCH / 3 : e.deltaMode === 2 ? NOTCH : 1);
    const notches = Math.max(-1, Math.min(1, -px / NOTCH));
    if (e.timeStamp - last > PAUSE || Math.sign(notches) !== Math.sign(rest)) rest = 0;
    last = e.timeStamp;
    const was = get(), got = zoomBy(was, notches + rest);
    rest = got.rest;
    if (got.view !== was.view || got.tile !== was.tile) set({ view: got.view, tile: got.tile });
    return true;
  };
}
