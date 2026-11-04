// Doppelte Dateien: groups of true copies below a folder, and the copies picked go to the trash.
import { action, inArchive } from '@/core/actions';
import { view } from '@/core/views';
import { win } from '@/core/windows';
import { t, tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import DupsBody from './DupsBody.vue';
import './duplicates.css';

export default function setup() {
  const F = usePanels();
  action('duplicates', { label: tl('duplicates.title'), g: 'IconCopy', run: () => {
    const here = F.T().path;
    const root = view.id === 'files' && !inArchive(here) ? here : '/mnt/user';
    win({ title: t('duplicates.title'), wide: true, cls: 'bk bk-dups', body: DupsBody, props: { root } });
  } });
}
