// Ansehen (F3) and the preview bar: one renderer per kind of file, in a large window or compact beside the list.
import './viewer.css';
import { action, actions, run } from '@/core/actions';
import { prefs, setPref } from '@/core/prefs';
import { go, registerView } from '@/core/views';
import { closeBtn, type FootButton, win } from '@/core/windows';
import { shown } from '@/files/entries';
import { inArchive } from '@/files/paths';
import { byName, t, tl } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { office, type OfficeSetup, rawUrl } from './content';
import { editable, type FileNode, joinPath, viewKind } from './kinds';
import OfficePage from './OfficePage.vue';
import ViewerBody from './ViewerBody.vue';

// The editor fills this in, so the viewer's Bearbeiten button opens it without the two importing each other.
export const hooks: { edit: ((n: FileNode, folder: string) => void) | null } = { edit: null };

let suite: OfficeSetup = { kind: 'none' };
export const officeName = (s = suite) => t(`viewer.office.kinds.${s.kind}`);

const shownHere = (k: FileNode) => shown(prefs.showHidden, prefs.showSystem)(k);

// The pictures and songs of a folder, in the order the panel shows them when it is the panel's own folder.
function siblings(folder: string, node: FileNode): FileNode[] {
  const F = usePanels(), kind = viewKind(node);
  const list: FileNode[] = folder === F.T().path
    ? F.entries().filter((k) => !k.up && (!k.at || k.at === folder))
    : [...(useListing().lists.get(folder)?.items || [])].sort((a, b) => byName(a.name, b.name));
  return list.filter((k) => !k.dir && viewKind(k) === kind && shownHere(k));
}

export function current(): { node: FileNode; folder: string } | null {
  const F = usePanels();
  const c = F.cursorEntry();
  const k = c && !c.up && !c.dir ? c : F.sel().find((x) => !x.dir);
  return k ? { node: k, folder: k.at || F.T().path } : null;
}

export function download(n: FileNode, folder: string) {
  const a = Object.assign(document.createElement('a'), { href: rawUrl(joinPath(folder, n.name), true), download: n.name });
  document.body.appendChild(a);
  a.click();
  a.remove();
}

// The office suite opens in a tab of its own, so the document has the whole window and the list stays here.
export function openInOffice(path: string) {
  const url = new URL(location.href);
  url.hash = '';
  url.searchParams.set('office', path);
  open(url.href, '_blank', 'noopener');
}

export function openViewer(node: FileNode, folder: string) {
  const kind = viewKind(node);
  const sibs = ['img', 'audio'].includes(kind) ? siblings(folder, node) : [node];
  if (!sibs.some((s) => s.name === node.name)) sibs.unshift(node);
  let cur = node;
  const inArch = inArchive(folder);
  const save: FootButton = { label: t('viewer.download'), g: 'IconDownload', run: () => { download(cur, folder); return false; } };
  const primary: FootButton = editable(node) && !inArch && hooks.edit
    ? { label: t('viewer.edit'), g: 'IconPencil', primary: true, run: () => { setTimeout(() => hooks.edit?.(cur, folder), 0); } }
    : kind === 'office' && suite.kind !== 'none'
      ? { label: t('viewer.openIn', { name: officeName() }), g: 'IconExternalLink', primary: true, run: () => { openInOffice(joinPath(folder, cur.name)); return false; } }
      : kind === 'arch' && actions.unpack && !inArch
        ? { label: t('viewer.unpack'), g: 'IconArchive', primary: true, run: () => { setTimeout(() => run('unpack'), 0); } }
        : { ...save, primary: true };
  const foot: FootButton[] = [
    { start: ' ' }, closeBtn(),
    { label: t('viewer.share'), g: 'IconLink', run: () => { run('share'); return false; } },
    ...(primary.label === save.label ? [] : [save]), primary,
  ];
  return win({
    title: node.name, full: true, cls: `cv-win cvk-${kind}`, body: ViewerBody, foot,
    props: { node, folder, sibs, onCurrent: (n: FileNode) => { cur = n; } },
  });
}

export default function setup() {
  office().then((s) => { suite = s; });
  action('view', { label: tl('viewer.title'), g: 'IconEye', key: 'F3', needs: 'file', run: () => {
    const c = current();
    if (c) openViewer(c.node, c.folder);
  } });
  action('quickview', { label: tl('viewer.quickview'), g: 'IconEye', key: 'Ctrl+q', checked: () => !!prefs.quickview, run: () => {
    if (!prefs.dual) setPref('dual', true);
    setPref('quickview', !prefs.quickview);
  } });
  registerView('office', { page: true, component: OfficePage });
  const asked = new URLSearchParams(location.search).get('office');
  if (asked) go('office', asked);
}
