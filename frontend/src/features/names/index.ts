// Neuer Ordner (F7), Neue Datei (Umschalt+F4) and Umbenennen (F2). The server keeps each as an undo step, so Strg+Z
// takes it back without a toast asking first.
import { reactive } from 'vue';
import { type FsChanged, ops } from '@/api/ops';
import { action, actions, run } from '@/core/actions';
import { toast } from '@/core/toast';
import { cancelBtn, win } from '@/core/windows';
import { join } from '@/files/paths';
import { t, tl } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import NameBody from './NameBody.vue';

function nameWindow(title: string, value: string, verb: string, glyph: string, apply: (name: string) => Promise<FsChanged>, old?: string) {
  const F = usePanels(), listing = useListing(), folder = F.T().path, panel = F.active;
  const taken = (name: string) => !!listing.lists.get(folder)?.items.some((k) => k.name === name);
  const st = reactive({ name: value });
  win({
    title, body: NameBody, props: { st },
    foot: [cancelBtn(), { label: verb, g: glyph, primary: true, run: () => {
      const name = st.name.trim();
      if (!name) return false;
      if (name !== old && taken(name)) { toast(t('names.taken'), 'warn'); return false; }
      if (name === old) return true;
      apply(name).then(async () => {
        await listing.load(folder).catch(() => null);
        const idx = F.entries(panel).findIndex((e) => e.name === name);
        if (idx >= 0 && F.T(panel).path === folder) F.T(panel).cursor = idx;
      }, (e) => toast(e.message, 'warn'));
      return true;
    } }],
  });
}

export default function setup() {
  const F = usePanels();
  const here = () => F.T().path;

  action('newFolder', { label: tl('names.newFolder'), g: 'IconFolderAdd', key: 'F7', needs: 'writable', run: () => {
    const folder = here();
    nameWindow(t('names.newFolder'), t('names.newFolder'), t('names.create'), 'IconFolderAdd', (name) => ops.mkdir(join(folder, name)));
  } });
  // The editor brings its own Neue Datei, which opens the file right away; this one only makes it.
  if (!actions.newFile) {
    action('newFile', { label: tl('names.newFile'), g: 'IconFileText', key: 'Umschalt+F4', needs: 'writable', run: () => {
      const folder = here();
      nameWindow(t('names.newFile'), t('names.newFileName'), t('names.create'), 'IconFileText', (name) => ops.touch(join(folder, name)));
    } });
  }
  action('rename', { label: tl('names.rename'), g: 'IconPencil', key: 'F2', needs: ['sel', 'writable'], run: () => {
    const sel = F.sel();
    if (sel.length > 1 && actions.multiRename) return run('multiRename');
    const k = sel[0]!, folder = k.at || here();
    return nameWindow(t('names.rename'), k.name, t('names.rename'), 'IconPencil', (name) => ops.rename(join(folder, k.name), name), k.name);
  } });
}
