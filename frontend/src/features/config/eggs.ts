// Three hidden looks, each opened by repeating one gesture five times: the leaf on the corners, the storm on the
// motion and disco on the rainbow. Whether one was found lives here and not in the prefs, so it is offered only while
// it is chosen or until the page is reloaded.
import { discoTap, leafTap, stormTap } from '@glimstone/core';
import { reactive } from 'vue';

export const found = reactive({ leaf: false, storm: false, disco: false });
const taps = { leaf: { taps: 0 }, storm: { taps: 0 }, disco: { taps: 0, last: 0 } };

// The press is read before the choice is saved, because it counts presses on the level already chosen.
export function shapePressed(pressed: string, current: string): string {
  if (!leafTap(taps.leaf, pressed, current)) return pressed;
  found.leaf = true;
  return 'leaf';
}

export function motionPressed(pressed: string, current: string): string {
  if (!stormTap(taps.storm, pressed, current)) return pressed;
  found.storm = true;
  return 'storm';
}

// Five turn-ons, each within three seconds of the last, so comparing the rainbow on and off now and then never opens
// disco by accident. Only turn-ons count, so the gesture ends with the rainbow on.
export function rainbowSwitched(on: boolean, now = Date.now()) {
  if (discoTap(taps.disco, on, { now })) found.disco = true;
}

export function forgetEggs() {
  Object.assign(found, { leaf: false, storm: false, disco: false });
  Object.assign(taps, { leaf: { taps: 0 }, storm: { taps: 0 }, disco: { taps: 0, last: 0 } });
}
