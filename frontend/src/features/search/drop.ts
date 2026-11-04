// The search field's menu opens when the field gets the focus and goes when it loses it, in whichever panel.
import { reactive } from 'vue';

export const EXAMPLES = [
  ['*.mkv', 'mkv'],
  ['#steuer', 'tag'],
  ['bericht', 'word'],
  ['IMG_????.jpg', 'one'],
  ['breite:>=3840 codec:hevc', 'meta'],
  ['geändert:<7d', 'week'],
] as const;

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
