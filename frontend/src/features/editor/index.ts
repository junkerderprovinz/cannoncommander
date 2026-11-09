// Bearbeiten (F4) and Neue Datei (Shift+F4): a text editor with a tab for each file, every common language coloured,
// Notepad++'s line commands, find and replace with regular expressions and a markdown preview. In the desktop app
// F4 may open another program instead, which desktop.ts hands in.
import './editor.css';
import { defineAsyncComponent } from 'vue';
import { action } from '@/core/actions';
import { bindKey, inScope } from '@/core/keymap';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { go, view } from '@/core/views';
import { type WinEntry, win } from '@/core/windows';
import { current, hooks, officeName, openInOffice } from '@/features/viewer';
import { office, touch } from '@/features/viewer/content';
import { editable, type FileNode, joinPath, viewKind } from '@/features/viewer/kinds';
import { t, tl } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { type Doc, makeDoc, newFile, readFile } from './docs';
import NameBody from './NameBody.vue';
import { actionId, EDITOR_COMMANDS, editorHost } from './session';

// CodeMirror comes in a chunk of its own, fetched right after start, so the first paint does not wait for it and
// F4 does not either.
const loadEditor = () => import('./EditorBody.vue');
const EditorBody = defineAsyncComponent(loadEditor);

// Another program for F4, which only the desktop app can start. It answers whether it took the file.
export const programHooks: { edit: ((path: string) => Promise<boolean>) | null } = { edit: null };

async function readDoc(node: FileNode, folder: string, fresh: boolean): Promise<Doc | null> {
  const r = fresh ? newFile() : await readFile(joinPath(folder, node.name)).catch(() => null);
  if (!r) {
    toast(t('editor.readFailed', { name: node.name }), 'fault');
    return null;
  }
  return makeDoc(node.name, folder, r);
}

// The text comes read already, so the editor stands ready the moment its window opens. With the window open the
// files join it as tabs.
export async function openEditor(nodes: FileNode[], folder: string, fresh = false): Promise<WinEntry | null> {
  const docs = (await Promise.all(nodes.map((n) => readDoc(n, folder, fresh)))).filter((d): d is Doc => !!d);
  if (!docs.length) return null;
  const open = editorHost();
  if (open) {
    open.add(docs);
    return null;
  }
  return win({
    title: docs[0]!.name, full: true, cls: 'ce-win', body: EditorBody, props: { docs },
    foot: [
      { start: ' ' },
      { label: t('core.close'), g: 'IconClose', wayOut: true, run: () => (editorHost()?.leave() === false ? false : undefined) },
      { label: t('editor.save'), g: 'IconSave', primary: true, run: () => { editorHost()?.run('save'); return false; } },
    ],
    // A click beside the window would close it; with unsaved text it asks first.
    onOpen: (el) => el.addEventListener('mousedown', (e) => {
      if (e.target === el && editorHost()?.leave() === false) e.stopImmediatePropagation();
    }, true),
  });
}

// F4 on a file: the program the desktop app was given for it, else the editor. Several marked files open as tabs.
async function edit(node: FileNode, folder: string, more: FileNode[] = []) {
  if (more.length < 2 && (await programHooks.edit?.(joinPath(folder, node.name)))) return;
  return openEditor(more.length > 1 ? more : [node], folder);
}

export default function setup() {
  loadEditor();
  hooks.edit = (n, folder) => { void edit(n, folder); };

  action('edit', { label: tl('editor.edit'), g: 'IconPencil', key: 'F4', needs: ['file', 'writable'], run: async () => {
    const c = current();
    if (!c) return;
    if (viewKind(c.node) === 'office') {
      const suite = await office();
      if (suite.kind === 'none') return toast(t('editor.officeMissing'), 'warn');
      toast(t('editor.officeOpens', { suite: officeName(suite), name: c.node.name }));
      return openInOffice(joinPath(c.folder, c.node.name));
    }
    if (!editable(c.node)) return toast(t('editor.viewOnly'), 'warn');
    const marked = usePanels().sel().filter((k) => !k.dir && !k.up && editable(k) && (k.at || usePanels().T().path) === c.folder);
    void edit(c.node, c.folder, marked);
  } });

  action('newFile', { label: tl('editor.newFile'), g: 'IconFileText', key: 'Shift+F4', needs: 'writable', run: () => {
    if (view.id !== 'files' && !editorHost()) go('files');
    const F = usePanels(), listing = useListing();
    // In the editor a new file goes beside the one in front, else into the panel's folder.
    const folder = editorHost()?.folder() ?? F.T().path;
    const value = t('editor.newName');
    win({
      title: t('editor.newFile'), body: NameBody, props: { value },
      foot: [{ label: t('editor.create'), g: 'IconFileText', primary: true, run: (el) => {
        const name = (el.querySelector<HTMLInputElement>('#nm')?.value || '').trim();
        if (!name) return false;
        if (listing.lists.get(folder)?.items.some((k) => k.name === name)) {
          toast(t('editor.exists'), 'warn');
          return false;
        }
        touch(joinPath(folder, name)).then(async () => {
          listing.invalidate(folder);
          await listing.load(folder).catch(() => null);
          const idx = F.entries().findIndex((k) => k.name === name);
          if (idx >= 0 && F.T().path === folder) F.T().cursor = idx;
          if (!editorHost() && (await programHooks.edit?.(joinPath(folder, name)))) return;
          void openEditor([{ name, size: 0, time: Date.now() }], folder, true);
        }, () => toast(t('editor.createFailed', { name }), 'fault'));
        return undefined;
      } }],
    });
  } });

  // Every command of the editor is an action of its own, so the keys tile lists it and it can take any key.
  for (const c of EDITOR_COMMANDS) {
    const id = actionId(c.name);
    action(id, {
      label: tl(`editor.cmd.${c.name}`), g: c.g, scope: 'editor', key: c.keys?.[0] && inScope('editor', c.keys[0]),
      checked: c.name === 'wrap' ? () => !!prefs.editorWrap : undefined,
      run: () => editorHost()?.run(c.name),
    });
    for (const k of c.keys ?? []) bindKey(inScope('editor', k), id);
  }
}
