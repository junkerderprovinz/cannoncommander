// How the app handles, picked in the welcome window at the first start of the desktop app. The explorer way shows one
// panel, marks a file with a click and several with a frame, and brings the keys of the system's own file manager. The
// commander way shows two panels, a click moves the cursor and the F-keys copy and move.
import { setPref } from '@/core/prefs';
import { useLayout } from '@/features/keys';

export type Style = 'explorer' | 'commander';

export function useStyle(style: Style) {
  setPref('desktopStyle', style);
  setPref('dual', style === 'commander');
  useLayout(style === 'explorer' ? 'explorer' : 'cc');
}
