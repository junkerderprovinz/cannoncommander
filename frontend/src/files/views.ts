// The ways a folder can be shown. Each folder remembers its own, so photos stay a gallery and logs stay a list.
import { parent } from './paths';

export const VIEWS: [id: string, glyph: string][] = [
  ['details', 'IconViewAdvanced'], ['compact', 'IconViewSimple'], ['list', 'IconLog'], ['content', 'IconFileDoc'],
  ['columns', 'IconCompare'], ['tiles', 'IconTabApps'], ['large', 'IconFileImage'], ['gallery', 'IconFileRaw'], ['drives', 'IconDatabase'],
];
export const CARD_VIEWS = ['tiles', 'large', 'gallery', 'drives'];
// Only these two show the column header.
export const ROW_VIEWS = ['details', 'compact'];

// A folder without its own choice opens where the volumes live as drives, and below a column view as columns, as in
// the Finder.
export function viewOf(path: string, folderViews: Record<string, string> = {}, holdsVolumes = false) {
  const own = folderViews[path];
  if (own) return own;
  if (holdsVolumes) return 'drives';
  for (let p = path; p !== '/'; p = parent(p)) if (folderViews[parent(p)] === 'columns') return 'columns';
  return 'details';
}

export const viewGlyph = (id: string) => (VIEWS.find((v) => v[0] === id) ?? VIEWS[0]!)[1];
