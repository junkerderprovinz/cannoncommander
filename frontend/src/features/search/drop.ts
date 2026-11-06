// The search field's menu opens when the field gets the focus and goes when it loses it, in whichever panel.
import { reactive } from 'vue';
import { t } from '@/i18n';

// What the menu offers to try, with the key that says what each finds. They are written in the interface's words.
export function examples(): [query: string, say: string][] {
  const field = (id: string) => t(`searchWords.${id}`);
  return [
    ['*.mkv', 'mkv'],
    [`#${t('search.exampleWords.tag')}`, 'tag'],
    [t('search.exampleWords.word'), 'word'],
    ['IMG_????.jpg', 'one'],
    [`${field('width')}:>=3840 ${field('vcodec')}:hevc`, 'meta'],
    [`${field('modified')}:<7d`, 'week'],
  ];
}

export const drop = reactive({ panel: -1 });

const panelOf = (el: EventTarget | null) => {
  const input = el instanceof HTMLInputElement && el.matches('.qfilter input') ? el : null;
  return input ? Number(input.id.replace('qf-', '')) : -1;
};

export function followFocus() {
  document.addEventListener('focusin', (e) => {
    const i = panelOf(e.target);
    if (i >= 0) drop.panel = i;
  });
  document.addEventListener('focusout', (e) => {
    if (panelOf(e.target) === drop.panel) drop.panel = -1;
  });
}
