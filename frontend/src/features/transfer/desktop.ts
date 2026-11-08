// Files shared with the other programs on the computer: its clipboard, which a paste takes from first, and drag and
// drop in both directions.
import { action } from '@/core/actions';
import { tl } from '@/i18n';
import { reachOutside } from './index';
import * as outside from './outside';

export default function setup() {
  reachOutside(outside);
  action('dropIn', { label: tl('transfer.copy'), g: 'IconCopy', hidden: true, run: outside.dropIn });
  action('dragOut', { label: tl('transfer.copy'), g: 'IconCopy', hidden: true, run: outside.dragOut });
  // Another program may have put files on the computer's clipboard while the window was in the background.
  window.addEventListener('focus', outside.lookSoon);
  void outside.readClipboard();
}
