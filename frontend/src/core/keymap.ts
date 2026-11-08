// Which key runs which action. The F-key bar and the shortcut window read this map, so keys of your own show up there too.
import { reactive } from 'vue';

export const DEFAULT_KEYS: Record<string, string> = {
  'Ctrl+1': 'single', 'Ctrl+2': 'dual', 'Ctrl+b': 'sidebar', 'Ctrl+q': 'quickview', 'Ctrl+h': 'hidden', 'Ctrl+u': 'swapPanels', 'Ctrl+.': 'equalPanels',
  'Ctrl+r': 'refresh', 'Ctrl+t': 'newTab', 'Ctrl+n': 'newTab', 'Ctrl+w': 'closeTab', 'Alt+ArrowLeft': 'back', 'Alt+ArrowRight': 'forward', 'Ctrl+l': 'gotoPath', 'Ctrl+k': 'palette',
  'Ctrl+a': 'selectAll', '*': 'invertSel', '-': 'unselect', '+': 'selectPattern', '/': 'quickFilter', 'Ctrl+f': 'quickFilter',
  'Ctrl+x': 'clipCut', 'Ctrl+c': 'clipCopy',
  F1: 'shortcuts', F2: 'rename', F3: 'view', F4: 'edit', 'Shift+F4': 'newFile', F5: 'copy', F6: 'move', F7: 'newFolder', F8: 'delete', F9: 'search', F10: 'menuBar', Delete: 'delete', 'Ctrl+Shift+n': 'newFolder',
  'Ctrl+d': 'duplicate', 'Shift+F7': 'folderWithSel', 'Alt+1': 'focusSide', 'Alt+2': 'focusLeft', 'Alt+3': 'focusRight', 'Alt+4': 'focusJobs',
  'Alt+F7': 'search', 'Alt+ArrowDown': 'history', 'Ctrl+m': 'multiRename', 'Ctrl+Shift+c': 'compareDirs', 'Ctrl+Shift+v': 'nextView', 'Ctrl+Shift+b': 'flat', 'Alt+Enter': 'props', 'Alt+p': 'pack', 'Alt+u': 'unpack', 'Alt+d': 'diskUsage',
};

export const keymap = reactive<Record<string, string>>({});
export const bindKey = (combo: string, id: string) => { keymap[combo] = id; };
export const bindDefaults = () => Object.entries(DEFAULT_KEYS).forEach(([k, id]) => bindKey(k, id));

// Shift counts for named keys and with Ctrl or Alt; on its own it only picks the character, such as * or +.
export const comboOf = (e: Pick<KeyboardEvent, 'ctrlKey' | 'metaKey' | 'altKey' | 'shiftKey' | 'key'>) => {
  const mod = e.ctrlKey || e.metaKey || e.altKey;
  return [e.ctrlKey || e.metaKey ? 'Ctrl' : '', e.altKey ? 'Alt' : '', e.shiftKey && (e.key.length > 1 || mod) ? 'Shift' : '', e.key.length === 1 ? e.key.toLowerCase() : e.key].filter(Boolean).join('+');
};

export const FKEYS = ['F1', 'F2', 'F3', 'F4', 'F5', 'F6', 'F7', 'F8', 'F9', 'F10'];
