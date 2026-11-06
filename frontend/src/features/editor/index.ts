// Bearbeiten (F4) and Neue Datei (Shift+F4): a text editor with line numbers, find and replace and a markdown preview.
import './editor.css';
import { defineAsyncComponent, reactive } from 'vue';
import { action } from '@/core/actions';
import { toast } from '@/core/toast';
import { go, view } from '@/core/views';
import { closeBtn, type WinEntry, win } from '@/core/windows';
import { current, hooks, officeName, openInOffice } from '@/features/viewer';
import { fsText, office, touch } from '@/features/viewer/content';
import { editable, type FileNode, joinPath, viewKind } from '@/features/viewer/kinds';
import { esc, glyphSvg } from '@/features/viewer/markdown';
import { t, tl } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import NameBody from './NameBody.vue';
import type { EditorState } from './state';

// CodeMirror comes in a chunk of its own, fetched right after start, so the first paint does not wait for it and
// F4 does not either.
const loadEditor = () => import('./EditorBody.vue');
const EditorBody = defineAsyncComponent(loadEditor);

export async function openEditor(node: FileNode, folder: string, fresh = false): Promise<WinEntry | null> {
  const file = fresh ? { text: '', encoding: 'utf-8' } : await fsText(joinPath(folder, node.name), 16 * 1024 * 1024, true).catch(() => null);
  if (!file) {
    toast(t('editor.readFailed', { name: node.name }), 'fault');
    return null;
  }
  const state: EditorState = reactive({ dirty: false, save: async () => false, ask: () => {} });
  let entry: WinEntry | null = null;
  const leave = () => setTimeout(() => entry?.close(), 0);
  state.ask = () => {
    win({
      title: t('editor.askTitle'),
      body: `<div class="glim-subcard"><div class="ce-ask">${glyphSvg('IconWarning')}<span>${t('editor.askText', { name: `<b>${esc(node.name)}</b>` })}</span></div></div>`,
      foot: [
        { label: t('editor.keepEditing'), g: 'IconPencil' },
        { label: t('editor.discard'), g: 'IconUndo', run: () => { state.dirty = false; leave(); } },
        { label: t('editor.save'), g: 'IconSave', primary: true, run: () => { state.save().then((ok) => ok && leave()); } },
      ],
    });
  };
  entry = win({
    title: node.name, full: true, cls: 'ce-win', body: EditorBody, props: { node, folder, file, state },
    foot: [
      { start: ' ' },
      { ...closeBtn(), run: () => (state.dirty ? (state.ask(), false) : undefined) },
      { label: t('editor.save'), g: 'IconSave', primary: true, run: () => { state.save(); return false; } },
    ],
    // A click beside the window would close it; with unsaved text it asks first.
    onOpen: (el) => el.addEventListener('mousedown', (e) => {
      if (e.target === el && state.dirty) {
        e.stopImmediatePropagation();
        state.ask();
      }
    }, true),
  });
  return entry;
}

export default function setup() {
  loadEditor();
  hooks.edit = (n, folder) => openEditor(n, folder);

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
    openEditor(c.node, c.folder);
  } });

  action('newFile', { label: tl('editor.newFile'), g: 'IconFileText', key: 'Shift+F4', needs: 'writable', run: () => {
    if (view.id !== 'files') go('files');
    const F = usePanels(), listing = useListing(), folder = F.T().path;
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
          openEditor({ name, size: 0, time: Date.now() }, folder, true);
        }, () => toast(t('editor.createFailed', { name }), 'fault'));
        return undefined;
      } }],
    });
  } });
}
