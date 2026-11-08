// Renaming in the row itself, as Explorer does on F2 or a slow second click. The names feature does the renaming;
// the list only knows which row shows the field.
import { reactive } from 'vue';

export const inline = reactive({ panel: -1, folder: '', name: '' });

type Save = (folder: string, from: string, to: string, panel: number) => Promise<unknown>;
let save: Save = async () => {};
export const onInlineSave = (fn: Save) => { save = fn; };

export const editing = (panel: number, folder: string, name: string) => inline.panel === panel && inline.folder === folder && inline.name === name;

export function startInline(panel: number, folder: string, name: string) {
  Object.assign(inline, { panel, folder, name });
}

export function endInline() {
  inline.panel = -1;
}

// Enter and leaving the field both keep what was typed, and Esc ends it unchanged.
export function saveInline(to: string) {
  if (inline.panel < 0) return Promise.resolve();
  const { panel, folder, name } = inline;
  endInline();
  const typed = to.trim();
  return typed && typed !== name ? save(folder, name, typed, panel) : Promise.resolve();
}

// The part of a name before its extension, which the field selects first. A dot at the start belongs to the name.
export const stemLength = (name: string, dir: boolean) => {
  const dot = name.lastIndexOf('.');
  return dir || dot <= 0 ? name.length : dot;
};
