// What happens to a drive: opening it, ejecting it, and the window following the computer when one comes or goes.
import { rootName } from '@/core/host';
import type { MenuItem } from '@/core/menu';
import { toast } from '@/core/toast';
import { driveName } from '@/files/drives';
import { folderItems } from '@/files/menus';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { ejectVolume, loadPlaces, places, reachVolume, type Volume, type VolumesEvent, within } from './store';

const say = (e: unknown) => toast(e instanceof Error ? e.message : String(e), 'warn');

export async function openVolume(v: Volume, panel?: number) {
  try {
    await usePanels().go(await reachVolume(v), panel);
  } catch (e) {
    say(e);
  }
}

// The panels on the drive move to the top of the computer at once, before the folder they showed is found missing,
// and says whether one did.
function leave(v: Volume): boolean {
  const F = usePanels();
  let moved = false;
  F.panels.forEach((p, i) => {
    if (within(p.tabs[p.cur]!.path, v)) {
      F.enter('/', i);
      moved = true;
    }
  });
  return moved;
}

// Explorer steps off a drive before it ejects it, so nothing of the window holds it.
export async function eject(v: Volume) {
  leave(v);
  try {
    await ejectVolume(v);
    toast(t(v.kind === 'optical' ? 'places.ejectedDisc' : 'places.ejected', { name: driveName(v) }));
  } catch (e) {
    say(e);
  }
}

export function driveMenu(v: Volume): MenuItem[] {
  const items = v.path ? folderItems(v.path) : [{ label: t('ctx.open'), g: 'IconFolderOpen', run: () => openVolume(v) }];
  return v.eject ? [...items, { label: t('places.eject'), g: 'IconEject', run: () => eject(v) }] : items;
}

export function volumesChanged(ev: VolumesEvent) {
  void loadPlaces().catch(() => {});
  for (const v of ev.removed) {
    if (leave(v)) toast(t('places.gone', { name: driveName(v), here: rootName() }), 'warn');
  }
  for (const v of ev.added) {
    toast(t('places.arrived', { name: driveName(v) }), 'ok', { label: t('places.open'), run: () => openVolume(v) });
  }
}

// The ejectable drive at path, for the rows that show drives without knowing them.
export const ejectableAt = (path: string) => places.volumes.find((v) => v.eject && v.path === path);
