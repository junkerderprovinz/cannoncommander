// How the app handles, asked once at the first start of the desktop app. As in Explorer, a click marks a file, a frame
// marks several and Explorer's keys work. As in a commander, a click moves the cursor and the F-keys copy and move.
import { prefs, setPref } from '@/core/prefs';
import { win } from '@/core/windows';
import { esc } from '@/features/viewer/markdown';
import { choosePreset } from '@/features/keys';
import { t } from '@/i18n';

export type Style = 'explorer' | 'commander';

export function useStyle(style: Style) {
  setPref('desktopStyle', style);
  setPref('clickSelect', style === 'explorer');
  choosePreset(style === 'explorer' ? 'explorer' : 'cc');
}

export function askStyle() {
  // The desktop app stands in for the computer's own file manager, which opens a file in its program.
  setPref('dblclick', 'system');
  const how = (key: string) => `<p class="dk-style"><b>${esc(t(`desktop.${key}Name`))}</b> ${esc(t(`desktop.${key}How`))}</p>`;
  win({
    title: t('desktop.styleTitle'),
    body: `<div class="glim-subcard"><p>${esc(t('desktop.styleText'))}</p>${how('explorer')}${how('commander')}</div>`,
    foot: [
      { label: t('desktop.explorerName'), g: 'IconFolderOpen', run: () => useStyle('explorer') },
      { label: t('desktop.commanderName'), g: 'IconPaneTwo', primary: true, run: () => useStyle('commander') },
    ],
    // Closed without an answer, it stays as it ships and does not ask again.
    onClose: () => { if (!prefs.desktopStyle) useStyle('commander'); },
  });
}
