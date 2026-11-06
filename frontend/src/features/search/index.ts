// Suchen (Alt+F7) with every criterion Krusader's search window has, the saved searches that list their hits in a
// panel, and the menu of the search field in the path row.
import { h } from 'vue';
import { action, inArchive } from '@/core/actions';
import { addTo } from '@/core/extend';
import { prefs } from '@/core/prefs';
import { view } from '@/core/views';
import { win } from '@/core/windows';
import { kindLabels, placeName } from '@/features/jobs/label';
import { toEntry } from '@/files/entries';
import { t, tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { BLANK, useSearch } from '@/stores/search';
import { drop, followFocus } from './drop';
import { saveFilter } from './saved';
import SearchBody from './SearchBody.vue';
import SearchDrop from './SearchDrop.vue';
import MetaWarn from './MetaWarn.vue';
import SmartBar from './SmartBar.vue';
import './search.css';

export default function setup() {
  const F = usePanels(), search = useSearch();
  search.load().catch(() => {});

  action('search', { label: tl('search.title'), g: 'IconSearch', key: 'Alt+F7', run: () => {
    const here = F.T().path;
    const start = view.id === 'files' && !inArchive(here) ? here : '/mnt/user';
    const q = { ...BLANK, hidden: !!prefs.showHidden, ...(search.last ?? {}), in: start };
    win({ title: t('search.title'), wide: true, cls: 'bk bk-search', body: SearchBody, props: { q } });
  } });
  action('saveSearch', { label: tl('search.saveFilter'), g: 'IconSave', when: () => !!F.T().filter.trim(), run: saveFilter });
  kindLabels.contentSearch = (j) => t('search.job', { name: t('search.quoted', { text: j.name }), to: placeName(j.dest) });

  // A panel showing a saved search lists its hits instead of the folder; any step elsewhere ends that.
  addTo('entryFilters', (list, _path, i) => {
    const s = search.byId(F.T(i).smart);
    return s ? search.hitsOf(s).items.map(toEntry) : list;
  });
  addTo('afterGo', (_path, i) => { F.T(i).smart = null; });
  addTo('tabLabels', (tab) => {
    const s = search.byId(tab.smart);
    return s ? { name: s.name, g: 'IconFilter' } : null;
  });
  addTo('panelBars', (i) => (F.T(i).smart ? h(SmartBar, { i }) : null));
  addTo('panelBars', (i) => h(MetaWarn, { i }));
  addTo('filterSlots', (i) => (drop.panel === i && !F.T(i).filter ? h(SearchDrop, { i }) : null));
  followFocus();
}
