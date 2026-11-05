// The folder picker: the server's tree in a window, and what a PathField needs to open it.
import { exists } from '@/api/admin';
import { win } from '@/core/windows';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
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

// The picker starts at the path in the field when that exists, otherwise where the active panel stands.
async function pickFrom(current: string, exts?: string[]) {
  const here = usePanels().T().path, typed = current.trim();
  const known = await exists(typed, exts ? 'file' : 'dir');
  return pickFolder({ start: known ? typed : here.includes('::') ? '/mnt/user' : here, exts });
}

// A PathField's props for a field with this id; with exts it picks files with one of those endings.
export const pathField = (id: string, exts?: string[]) => ({
  id,
  pick: (current: string) => pickFrom(current, exts),
  text: { pick: exts ? t('ui.pickFile') : t('ui.pickFolder') },
  buttonAttrs: { 'data-pickfor': id },
});
