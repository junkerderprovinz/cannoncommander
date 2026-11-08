// Saving a search under a name, opening one in the panel, and the right-click menu of its row in the search field.
import { nextTick, reactive } from 'vue';
import type { Hit, SavedSearch, SearchQuery } from '@/api/tools';
import { menu } from '@/core/menu';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { go as goView, view } from '@/core/views';
import { isHidden } from '@/files/entries';
import { cancelBtn, win, winStack } from '@/core/windows';
import { split } from '@/files/paths';
import { t } from '@/i18n';
import { newTab, usePanels } from '@/stores/panels';
import { BLANK, useSearch } from '@/stores/search';
import NameBody from './NameBody.vue';

function nameWindow(title: string, value: string, verb: string, done: (name: string) => void) {
  const st = reactive({ name: value });
  win({
    title, body: NameBody, props: { st },
    foot: [cancelBtn(), { label: verb, g: 'IconSave', primary: true, run: () => {
      const name = st.name.trim();
      if (!name) return false;
      done(name);
      return true;
    } }],
  });
}

const warn = (e: unknown) => toast((e as Error).message, 'warn');

const suggest = (q: SearchQuery) => q.name.trim() || (q.text.trim() ? t('search.suggestText', { text: q.text.trim() }) : t('search.suggestIn', { name: split(q.in).pop() || t('search.server') }));

export async function openSaved(s: SavedSearch, asTab = false) {
  const F = usePanels(), search = useSearch();
  if (view.id !== 'files') goView('files');
  if (asTab) {
    const p = F.P();
    p.tabs.push(newTab(s.query.in));
    p.cur = p.tabs.length - 1;
  }
  await F.go(s.query.in, F.active, !asTab);
  if (F.T().path !== s.query.in) return;
  search.refresh(s);
  F.T().smart = s.id;
}

export function saveSearch(q: SearchQuery) {
  nameWindow(t('search.save'), suggest(q), t('search.saveVerb'), (name) => {
    useSearch().save(name, { ...q }).then((s) => {
      toast(t('search.savedToast', { name }), 'ok', { label: t('search.open'), run: () => { while (winStack.length) winStack.at(-1)!.close(); void openSaved(s); } });
    }, warn);
  });
}

export function saveFilter() {
  const tab = usePanels().T();
  saveSearch({ ...BLANK, in: tab.path, hidden: !!prefs.showHidden, name: tab.filter.trim() });
}

export function savedMenu(s: SavedSearch, x: number, y: number) {
  const search = useSearch();
  menu(x, y, [
    { label: t('search.open'), g: 'IconFolderOpen', run: () => openSaved(s) },
    { label: t('search.openTab'), g: 'IconAdd', run: () => openSaved(s, true) },
    '-',
    { label: t('search.rename'), g: 'IconPencil', run: () => nameWindow(t('search.renameTitle'), s.name, t('search.renameVerb'), (name) => { search.rename(s, name).catch(warn); }) },
    { label: t('search.remove'), g: 'IconTrash', run: () => {
      search.remove(s).then(() => {
        for (const p of usePanels().panels) for (const tab of p.tabs) if (tab.smart === s.id) tab.smart = null;
        toast(t('search.removed', { name: s.name }), 'ok', { label: t('core.undo'), run: () => { search.restore(s).catch(warn); } });
      }, warn);
    } },
  ]);
}

// A hit opens where it lives with the cursor on it; a hidden one first turns hidden files on, or it could not be seen.
export async function openHit(hit: Hit, asTab: boolean) {
  const F = usePanels();
  if (!prefs.showHidden && isHidden(hit)) setPref('showHidden', true);
  const dir = hit.at || '/';
  if (asTab) {
    const p = F.P();
    p.tabs.push(newTab(dir));
    p.cur = p.tabs.length - 1;
    await F.go(dir, F.active, false);
  } else await F.go(dir);
  reveal(dir, hit.name);
  await nextTick();
  document.querySelector('.panel.active .row.cursor')?.scrollIntoView({ block: 'center' });
}

// Puts the cursor on a name in every panel that shows its folder.
export function reveal(dir: string, name: string) {
  const F = usePanels();
  for (let i = 0; i < (prefs.dual ? 2 : 1); i++) {
    if (F.T(i).path !== dir) continue;
    const idx = F.entries(i).findIndex((e) => e.name === name);
    if (idx >= 0) F.T(i).cursor = idx;
  }
}
