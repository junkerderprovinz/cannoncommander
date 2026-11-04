// One keyboard service for the whole app, in the capture phase: windows and menus first, then the page, the key map
// and last the active view.
import { actions, run } from './actions';
import { comboOf, keymap } from './keymap';
import { closeMenu, isMenuOpen } from './menu';
import { openMenuBar } from './menus';
import { currentView, go } from './views';
import { closeWin, winStack } from './windows';

const typing = (t: EventTarget | null) => !!(t as Element | null)?.closest?.('input, textarea, select, [contenteditable]');

export function handleKey(e: KeyboardEvent) {
  if (isMenuOpen()) {
    if (e.key === 'Escape') { e.preventDefault(); closeMenu(); }
    return;
  }
  if (winStack.length) {
    if (e.key === 'Escape') { e.preventDefault(); closeWin(); }
    return;
  }
  // Escape closes an open popover first and leaves the page only when none is open.
  if (e.key === 'Escape' && currentView()?.page && !e.defaultPrevented && !document.querySelector('.a-cpop, .cpick') && !typing(e.target)) {
    e.preventDefault();
    go('files');
    return;
  }
  if (e.key === 'F10') { e.preventDefault(); openMenuBar(0); return; }
  if (typing(e.target)) return;
  const combo = comboOf(e);
  const id = keymap[combo];
  if (id && actions[id]) { e.preventDefault(); run(id); return; }
  currentView()?.key?.(e, combo);
}

export const installKeyboard = () => document.addEventListener('keydown', handleKey, true);
