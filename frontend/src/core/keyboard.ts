// One keyboard service for the whole app, in the capture phase: windows and menus first, then the page, the key map
// and last the active view.
import { actions, run } from './actions';
import { comboOf, keymap } from './keymap';
import { closeMenu, isMenuOpen } from './menu';
import { openMenuBar } from './menus';
import { currentView, go } from './views';
import { closeWin, winStack } from './windows';

const typing = (t: EventTarget | null) => !!(t as Element | null)?.closest?.('input, textarea, select, [contenteditable]');

// While something covers the whole app, such as the sign in after a session ran out, its keys stay with it.
let covered = () => false;
export const coverKeys = (fn: () => boolean) => { covered = fn; };

export function handleKey(e: KeyboardEvent) {
  if (covered()) return;
  if (isMenuOpen()) {
    if (e.key === 'Escape') { e.preventDefault(); closeMenu(); }
    return;
  }
  if (winStack.length) {
    if (e.key === 'Escape' && !e.defaultPrevented) { e.preventDefault(); if (!winStack.at(-1)!.locked) closeWin(); }
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
