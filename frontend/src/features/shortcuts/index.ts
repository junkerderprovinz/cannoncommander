// F1: every key the app knows, the fixed ones first and then those the actions bring.
import { action } from '@/core/actions';
import { closeBtn, win } from '@/core/windows';
import { t, tl } from '@/i18n';
import ShortcutsBody from './ShortcutsBody.vue';

export default function setup() {
  action('shortcuts', { label: tl('shortcuts.title'), g: 'IconKeyboard', key: 'F1', run: () => win({ title: t('shortcuts.title'), wide: true, body: ShortcutsBody, foot: [closeBtn()] }) });
}
