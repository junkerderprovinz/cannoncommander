// The folder picker: the server's tree in a window, resolving to the path chosen or null when it was left.
import { win } from '@/core/windows';
import { t } from '@/i18n';
import FolderPicker from './FolderPicker.vue';
import './texts';

export function pickFolder({ start, exts }: { start: string; exts?: string[] }) {
  return new Promise<string | null>((resolve) => {
    let done = false;
    win({
      title: exts ? t('ui.pickFile') : t('ui.pickFolder'), cls: 'a-picker', body: FolderPicker,
      props: { start, exts, done: (p: string) => { done = true; resolve(p); } },
      onClose: () => { if (!done) resolve(null); },
    });
  });
}
