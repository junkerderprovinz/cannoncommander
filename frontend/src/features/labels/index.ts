// Colours and tags on files and folders: the marks beside a name, the # menu that searches by them, the entries of
// the row menu and the windows that edit them.
import './labels.css';
import { h } from 'vue';
import { action } from '@/core/actions';
import { addTo } from '@/core/extend';
import { menu, type MenuEntry, type MenuItem } from '@/core/menu';
import { win } from '@/core/windows';
import { t, tl } from '@/i18n';
import { type Labelled, useLabels } from '@/stores/labels';
import { usePanels } from '@/stores/panels';
import ColoursWin from './ColoursWin.vue';
import Marks from './Marks.vue';
import TagEditWin from './TagEditWin.vue';
import TagsWin from './TagsWin.vue';

export const editColours = () => win({ title: t('labels.colours.title'), body: ColoursWin });
export const editTags = () => win({ title: t('labels.tags'), body: TagsWin });
export const editItemTags = (sel: Labelled[], folder: string) => win({
  title: sel.length === 1 ? t('labels.tagsFor', { name: sel[0]!.name }) : t('labels.tagsForN', { n: sel.length }),
  body: TagEditWin, props: { sel, folder },
});

// The row menu's colour and tag entries for the selection in a folder.
export function labelMenuItems(sel: Labelled[], folder: string): MenuItem[] {
  const labels = useLabels();
  const now = sel.length === 1 ? labels.of(sel[0]!, folder).colour : null;
  return [
    { label: t('labels.colour'), g: 'IconTabLook', sub: [
      ...labels.colours.map((x) => ({ label: now === x.id ? t('labels.isSet', { name: labels.colourName(x) }) : labels.colourName(x), color: x.hex, run: () => labels.set(sel, folder, { colour: x.id }) })),
      '-', { label: t('labels.noColour'), g: 'IconClose', run: () => labels.set(sel, folder, { colour: '' }) },
    ] },
    { label: t('labels.tags'), g: 'IconHash', run: () => editItemTags(sel, folder) },
  ];
}

// Every colour and tag with how many carry it. Picking one searches the shares for it.
export function labelsMenu(anchor: HTMLElement) {
  const labels = useLabels();
  const search = (word: string) => async () => {
    const F = usePanels();
    await F.go('/mnt/user');
    Object.assign(F.T(), { filter: word, qf: true, deep: true, cursor: 0 });
  };
  const tags: MenuEntry[] = labels.tags.map((x) => ({ label: x.name, g: 'IconHash', count: x.count || undefined, run: search('#' + x.name.toLowerCase()) }));
  const r = anchor.getBoundingClientRect();
  menu(r.left, r.bottom + 6, [
    { head: t('labels.colours.title'), g: 'IconPencil', tip: t('labels.editColours'), run: editColours },
    ...labels.colours.map((x) => ({ label: labels.colourName(x), color: x.hex, count: x.count || undefined, run: search('#' + labels.colourName(x).toLowerCase()) })),
    { head: t('labels.tags'), g: 'IconPencil', tip: t('labels.editTags'), run: editTags },
    ...(tags.length ? tags : [{ label: t('labels.noTagsYet'), g: 'IconInfo', run: editTags }]),
  ]);
}

export default function setup() {
  const labels = useLabels();
  labels.load().catch(() => {});
  addTo('nameBadges', (k, path) => h(Marks, { k, path }));
  addTo('tagMatchers', (k, word, path) => labels.matches(k, word, path));
  action('labelsMenu', {
    label: tl('labels.menu'), g: 'IconHash',
    run: (anchor?: HTMLElement) => {
      const at = anchor ?? document.querySelector<HTMLElement>('.panel.active [data-labelsmenu]') ?? document.querySelector<HTMLElement>('[data-labelsmenu]');
      if (at) labelsMenu(at);
    },
  });
  action('editLabelColours', { label: tl('labels.editColours'), g: 'IconTabLook', run: editColours });
  action('editLabelTags', { label: tl('labels.editTags'), g: 'IconHash', run: editTags });
}
