// Löschen (F8, Entf) into the trash or for good (Umschalt+Entf), and the Papierkorb page.
import { action } from '@/core/actions';
import { bindKey } from '@/core/keymap';
import { go, registerView } from '@/core/views';
import { tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { useTrash } from '@/stores/trash';
import { pick } from './pick';
import { deleteItems } from './trash';
import TrashPage from './TrashPage.vue';
import './trash.css';

export default function setup() {
  const F = usePanels();
  useTrash().load().catch(() => {});
  action('delete', { label: tl('trash.delete'), g: 'IconTrash', key: 'F8', needs: ['sel', 'writable'], run: () => deleteItems(F.sel(), F.T().path) });
  action('deleteNow', { label: tl('trash.deleteNow'), g: 'IconTrash', key: 'Umschalt+Entf', needs: ['sel', 'writable'], run: () => deleteItems(F.sel(), F.T().path, { forever: true }) });
  bindKey('Shift+Delete', 'deleteNow');
  registerView('trash', { page: true, component: TrashPage });
  action('openTrash', { label: tl('trash.title'), g: 'IconTrash', run: () => { pick.sel.clear(); go('trash'); } });
}
