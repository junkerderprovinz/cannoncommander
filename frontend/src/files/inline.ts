// Renaming in the row itself, on F2 or a slow second click. The names feature does the renaming; the list only knows
// which row shows the field.
import { reactive } from 'vue';

export const inline = reactive({ panel: -1, folder: '', name: '', error: '', saving: false });

// A save that cannot take the name throws, and its message stands at the field.
type Save = (folder: string, from: string, to: string, panel: number) => Promise<unknown>;
let save: Save = async () => {};
export const onInlineSave = (fn: Save) => { save = fn; };

export const editing = (panel: number, folder: string, name: string) => inline.panel === panel && inline.folder === folder && inline.name === name;

export function startInline(panel: number, folder: string, name: string) {
  Object.assign(inline, { panel, folder, name, error: '' });
}

export function endInline() {
  Object.assign(inline, { panel: -1, error: '' });
}

// Enter and leaving the field both keep what was typed, and Esc ends it unchanged. A name that cannot be taken
// keeps the field open with the typed name and the reason.
export async function saveInline(to: string) {
  if (inline.panel < 0 || inline.saving) return;
  const { panel, folder, name } = inline;
  const typed = to.trim();
  if (!typed || typed === name) return endInline();
  inline.saving = true;
  try {
    await save(folder, name, typed, panel);
    endInline();
  } catch (e) {
    inline.error = e instanceof Error ? e.message : String(e);
  } finally {
    inline.saving = false;
  }
}
