// The ways a folder can be shown. Each folder remembers its own, so photos stay a gallery and logs stay a list.
import { prefs } from '@/core/prefs';
import type { Tab } from '@/stores/panels';

export const VIEWS: [id: string, glyph: string][] = [
  ['details', 'IconViewAdvanced'], ['compact', 'IconViewSimple'], ['list', 'IconLog'], ['content', 'IconFileDoc'],
  ['columns', 'IconCompare'], ['tiles', 'IconTabApps'], ['large', 'IconFileImage'], ['gallery', 'IconFileRaw'], ['drives', 'IconDatabase'],
];
export const CARD_VIEWS = ['tiles', 'large', 'gallery', 'drives'];
// Only these two have rows yet; any other stored view shows as details.
const DRAWN = ['details', 'compact'];

export const viewOf = (t: Pick<Tab, 'path'>) => {
  const own = prefs.folderViews?.[t.path];
  return own && DRAWN.includes(own) ? own : 'details';
};
export const viewGlyph = (id: string) => VIEWS.find((v) => v[0] === id)![1];
