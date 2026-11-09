// What only the desktop app adds to the editor: F4 and Neue Datei may open text files in a program of the computer,
// which the general settings pick, and the built-in editor stays a menu entry away.
import { action } from '@/core/actions';
import { addOpenMenuItems } from '@/files/menus';
import { inArchive } from '@/files/paths';
import { current } from '@/features/viewer';
import { editable } from '@/features/viewer/kinds';
import { tl } from '@/i18n';
import { openEditor, programHooks } from './index';
import { editorProgram, openInProgram } from './program';

export default function setup() {
  programHooks.edit = async (path) => {
    const p = editorProgram();
    if (!p) return false;
    await openInProgram(p, path);
    return true;
  };
  action('editBuiltIn', {
    label: tl('editor.program.editBuiltIn'), g: 'IconPencil', needs: ['file', 'writable'], when: () => !!editorProgram(),
    run: () => {
      const c = current();
      if (c && editable(c.node)) void openEditor([c.node], c.folder);
    },
  });
  addOpenMenuItems((sel, path) => {
    const one = sel.length === 1 ? sel[0]! : null;
    return editorProgram() && one && !one.dir && !one.up && editable(one) && !inArchive(path) ? ['editBuiltIn'] : [];
  });
}
