// Orte in the desktop app: the computer's own folders above the pinned ones, the folders used last below them, and
// a card with the drives, which follows sticks and shares as they come and go.
import './places.css';
import { h, watch } from 'vue';
import { on } from '@/api/sse';
import { addTo } from '@/core/extend';
import { G } from '@/core/glyphs';
import { host } from '@/core/host';
import { addRowMenuItems } from '@/files/menus';
import { join } from '@/files/paths';
import { t } from '@/i18n';
import { eject, ejectableAt, volumesChanged } from './actions';
import DrivesCard from './DrivesCard.vue';
import KnownRows from './KnownRows.vue';
import RecentRows from './RecentRows.vue';
import { loadPlaces, visited, type VolumesEvent } from './store';

export default function setup() {
  addTo('sideTop', () => h(KnownRows));
  addTo('sideRows', () => h(RecentRows));
  addTo('sideCards', () => h(DrivesCard));
  addTo('afterGo', () => { visited.value++; });
  addTo('treeDecorators', (path) => {
    const v = ejectableAt(path);
    return v && h('button', {
      class: 'unpin', 'data-eject': v.id, 'data-tip': t('places.eject'), 'aria-label': t('places.eject'),
      onClick: (e: MouseEvent) => { e.stopPropagation(); void eject(v); },
    }, [h(G, { n: 'IconEject' })]);
  });
  addRowMenuItems((sel, path) => {
    const v = sel.length === 1 && sel[0]!.dir ? ejectableAt(join(path, sel[0]!.name)) : undefined;
    return v ? [{ label: t('places.eject'), g: 'IconEject', run: () => eject(v) }] : [];
  });
  on('desktop.volumes', (e) => volumesChanged(e.data as VolumesEvent));
  on('resync', () => { if (host.desktop) void loadPlaces().catch(() => {}); });
  watch(() => host.desktop, (desktop) => { if (desktop) void loadPlaces().catch(() => {}); }, { immediate: true });
}
