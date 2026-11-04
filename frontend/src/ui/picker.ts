// The folder picker: the server's tree in a window, resolving to the folder chosen or null when it was left.
import { reactive } from 'vue';
import { toast } from '@/core/toast';
import { cancelBtn, win } from '@/core/windows';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import FolderTree from './FolderTree.vue';
import './texts';

export function pickFolder(start: string): Promise<string | null> {
  const listing = useListing();
  return new Promise((resolve) => {
    const st = reactive({ path: start });
    let done = false;
    const choose = (p: string) => { done = true; resolve(p); };
    const w = win({
      title: t('ui.pickFolder'), cls: 'a-picker', body: FolderTree, props: { st },
      foot: [cancelBtn(), { label: t('ui.pick'), g: 'IconCheck', primary: true, run: () => {
        const p = st.path.trim().replace(/(.)\/$/, '$1') || '/';
        if (listing.lists.has(p)) return choose(p);
        // A folder typed by hand is read first, so the window stays open for one that is not there.
        listing.load(p).then(() => { choose(p); w.close(); }, () => toast(t('ui.noFolder'), 'warn'));
        return false;
      } }],
      onClose: () => { if (!done) resolve(null); },
    });
  });
}
