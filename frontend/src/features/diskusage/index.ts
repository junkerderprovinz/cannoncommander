// Speicherbelegung (Alt+D): where the room in a folder goes, as a tree, a treemap, by file type and as its largest files.
import { action, inArchive } from '@/core/actions';
import { view } from '@/core/views';
import { win } from '@/core/windows';
import { t, tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { countEnded } from './ended';
import UsageBody from './UsageBody.vue';
import './diskusage.css';

export default function setup() {
  const F = usePanels();
  countEnded();
  action('diskUsage', { label: tl('diskusage.title'), g: 'IconDashboard', key: 'Alt+D', run: (start?: string) => {
    const here = F.T().path;
    const from = typeof start === 'string' && start ? start : view.id === 'files' && !inArchive(here) ? here : '/mnt/user';
    win({ title: t('diskusage.title'), full: true, cls: 'bk bk-du', body: UsageBody, props: { start: from } });
  } });
}
