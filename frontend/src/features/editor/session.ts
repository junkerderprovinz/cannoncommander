// The editor window that is open, which the editor's named actions and keys work in. There is one at most: it covers
// the app, and opening another file adds a tab to it.
import type { Doc } from './docs';

export interface EditorHost {
  run: (command: string) => void;
  add: (docs: Doc[]) => void;
  // The folder of the file in front, where a new file goes.
  folder: () => string;
  // Whether the window may close now. With unsaved work it asks instead and answers false.
  leave: () => boolean;
}

let open: EditorHost | null = null;
export const editorHost = () => open;
export const setEditorHost = (h: EditorHost | null) => { open = h; };

// What every named action of the editor is: a command of the text, which needs the caret in it, or one of the window,
// which works from the find field as well. Those marked vim keep their key while Vim has the text.
export interface EditorCommand { name: string; g: string; keys?: string[]; text?: boolean; vim?: boolean }

export const EDITOR_COMMANDS: EditorCommand[] = [
  { name: 'save', g: 'IconSave', keys: ['Ctrl+s'], vim: true },
  { name: 'find', g: 'IconSearch', keys: ['Ctrl+f'] },
  { name: 'replace', g: 'IconSearch', keys: ['Ctrl+h'] },
  { name: 'findNext', g: 'IconMoveDown', keys: ['F3'] },
  { name: 'findPrev', g: 'IconMoveUp', keys: ['Shift+F3'] },
  { name: 'gotoLine', g: 'IconForward', keys: ['Ctrl+g'] },
  { name: 'duplicateLine', g: 'IconCopy', keys: ['Ctrl+d'], text: true },
  { name: 'deleteLine', g: 'IconTrash', keys: ['Ctrl+Shift+l'], text: true },
  { name: 'moveLineUp', g: 'IconMoveUp', keys: ['Ctrl+Shift+ArrowUp', 'Alt+ArrowUp'], text: true },
  { name: 'moveLineDown', g: 'IconMoveDown', keys: ['Ctrl+Shift+ArrowDown', 'Alt+ArrowDown'], text: true },
  { name: 'toggleComment', g: 'IconCode', keys: ['Ctrl+/', 'Ctrl+#'], text: true },
  { name: 'sortLines', g: 'IconSort', text: true },
  { name: 'sortLinesDown', g: 'IconSort', text: true },
  { name: 'trimTrailing', g: 'IconPrune', text: true },
  { name: 'upperCase', g: 'IconFont', keys: ['Ctrl+Shift+u'], text: true },
  { name: 'lowerCase', g: 'IconFont', keys: ['Ctrl+u'], text: true },
  { name: 'fold', g: 'IconCollapse', keys: ['Ctrl+Alt+f'], text: true },
  { name: 'unfold', g: 'IconExpand', keys: ['Ctrl+Alt+Shift+f'], text: true },
  { name: 'foldAll', g: 'IconCollapse', keys: ['Alt+0'], text: true },
  { name: 'unfoldAll', g: 'IconExpand', text: true },
  { name: 'wrap', g: 'IconLabelText' },
  { name: 'nextTab', g: 'IconForward', keys: ['Ctrl+Tab', 'Ctrl+PageDown', 'Alt+PageDown'], vim: true },
  { name: 'prevTab', g: 'IconBack', keys: ['Ctrl+Shift+Tab', 'Ctrl+PageUp', 'Alt+PageUp'], vim: true },
  { name: 'closeTab', g: 'IconClose', keys: ['Ctrl+w'], vim: true },
];

export const actionId = (name: string) => 'editor' + name[0]!.toUpperCase() + name.slice(1);
export const commandOf = (id: string) => EDITOR_COMMANDS.find((c) => actionId(c.name) === id);
