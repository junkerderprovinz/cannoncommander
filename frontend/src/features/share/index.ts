// Freigeben: one public link per file or folder, or one for a collection, which the other side sees as a folder. Die
// Sammelmappe gathers entries from several folders for such a link. Herunterladen sends a file, or a packed folder.
import { SidebarRow } from '@glimstone/vue';
import { h, reactive, watch } from 'vue';
import { on } from '@/api/sse';
import { type Share, tools } from '@/api/tools';
import { action, run } from '@/core/actions';
import { host } from '@/core/host';
import { addTo } from '@/core/extend';
import { fmtSize, now } from '@/core/format';
import { G, glyphComponent } from '@/core/glyphs';
import { MENUS } from '@/core/menus';
import { toast } from '@/core/toast';
import { cancelBtn, closeBtn, win } from '@/core/windows';
import type { Entry } from '@/files/entries';
import { inArchive, join, split } from '@/files/paths';
import { locale, t, tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { plural, setPrimary } from '@/ui/kit';
import BasketBody from './BasketBody.vue';
import { basket, type Collected } from './basket';
import DoneBody from './DoneBody.vue';
import DownloadBody, { type DownloadState } from './DownloadBody.vue';
import { create, newForm, readForm } from './link';
import ManyBody from './ManyBody.vue';
import ShareBody from './ShareBody.vue';
import './share.css';

const collected = (k: Entry, folder: string): Collected => ({ path: join(k.at || folder, k.name), name: k.name, dir: k.dir, size: k.size || 0 });

function shareWindow() {
  const F = usePanels(), sel = F.sel(), folder = F.T().path;
  if (!sel.length) return toast(t('share.pickFirst'), 'warn');
  if (inArchive(folder)) return toast(t('share.inArchive'), 'warn');
  // Several entries, or hits from a search below this folder, go out together under one link.
  if (sel.length > 1 || (sel[0]!.at && sel[0]!.at !== folder)) return shareMany(sel.map((k) => collected(k, folder)));
  const k = sel[0]!, path = join(folder, k.name);
  const f = reactive(newForm(k.name, k.dir));
  const st = reactive({ existing: [] as Share[] });
  const load = () => tools.shares().then((all) => { st.existing = all.filter((s) => s.kind !== 'collection' && s.paths[0] === path); }, () => {});
  void load();
  async function reroll(s: Share) {
    const old = s.slug;
    try {
      const fresh = await tools.changeShare(s.id, { reroll: true });
      Object.assign(s, fresh);
      toast(t('share.rerolled'), 'ok', { label: t('core.undo'), run: () => { tools.changeShare(s.id, { slug: old }).then((back) => Object.assign(s, back), () => {}); } });
    } catch (e) {
      toast((e as Error).message, 'warn');
    }
  }
  win({
    title: t('share.share'), body: ShareBody, props: { k, f, st, reroll },
    foot: [{ start: path }, cancelBtn(), { label: t('share.create'), g: 'IconLink', primary: true, run: () => {
      const req = readForm(f);
      if (!req) return false;
      create({ paths: [path], ...req }).then((link) => { if (link) done(link, k.dir); });
      return true;
    } }],
  });
}

function shareMany(picked: Collected[], fromBasket = false) {
  const items = picked.filter((x) => !inArchive(x.path));
  if (!items.length) return toast(t('share.inArchive'), 'warn');
  const f = reactive(newForm('sammlung', false));
  const st = reactive({ title: t('share.suggest', { date: new Date(now()).toLocaleDateString(locale(), { day: '2-digit', month: '2-digit' }) }) });
  win({
    title: t('share.together'), body: ManyBody, props: { items, f, st },
    foot: [{ start: t('share.oneLink') }, cancelBtn(), { label: t('share.create'), g: 'IconLink', primary: true, run: (el) => {
      const name = st.title.trim();
      if (!name) { toast(t('share.nameIt'), 'warn'); el.querySelector<HTMLInputElement>('#cs-title')?.focus(); return false; }
      const req = readForm(f);
      if (!req) return false;
      create({ paths: items.map((x) => x.path), name, ...req }).then((link) => {
        if (!link) return;
        if (fromBasket) basket.splice(0);
        done(link, true, items);
      });
      return true;
    } }],
  });
}

function done(link: Share, dir: boolean, items?: Collected[]) {
  win({
    title: t('share.created'), wide: !!items, body: DoneBody, props: { link, dir, items },
    foot: [{ label: t('share.remove'), g: 'IconUnlink', run: () => { tools.dropShare(link.id).then(() => toast(t('share.removed')), (e) => toast(e.message, 'warn')); } },
      { label: t('share.done'), g: 'IconCheck', primary: true }],
  });
}

function collect() {
  const F = usePanels(), sel = F.sel(), folder = F.T().path;
  if (!sel.length) return toast(t('share.pickFirst'), 'warn');
  if (inArchive(folder)) return toast(t('share.inArchive'), 'warn');
  const fresh = sel.map((k) => collected(k, folder)).filter((x) => !basket.some((b) => b.path === x.path));
  basket.push(...fresh);
  toast(fresh.length ? t('share.collected', { objects: plural(fresh.length, 'ui.objects'), n: basket.length }) : t('share.already'), 'ok', { label: t('share.collection'), run: () => run('collection') });
}

function collectionWindow() {
  let stop = () => {};
  const w = win({
    title: t('share.collection'), wide: true, body: BasketBody,
    foot: [{ start: t('share.stay') }, { label: t('share.clear'), g: 'IconEmpty', run: () => { basket.splice(0); return false; } }, closeBtn(),
      { label: t('share.shareAll'), g: 'IconLink', primary: true, disabled: !basket.length, run: () => {
        if (!basket.length) return false;
        const items = basket.slice();
        setTimeout(() => shareMany(items, true), 0);
        return true;
      } }],
    onClose: () => stop(),
  });
  stop = watch(() => basket.length, (n) => setPrimary(w, { disabled: !n }));
}

function download() {
  const F = usePanels(), sel = F.sel(), folder = F.T().path;
  const paths = sel.map((k) => join(k.at || folder, k.name));
  if (sel.length === 1 && !sel[0]!.dir) {
    const a = Object.assign(document.createElement('a'), { href: tools.rawUrl(paths[0]!), download: sel[0]!.name });
    document.body.append(a);
    a.click();
    a.remove();
    return toast(t('share.downloading', { name: sel[0]!.name, size: fmtSize(sel[0]!.size) || '0 B' }));
  }
  const base = (sel.length === 1 ? sel[0]!.name : split(folder).pop() || 'dateien').replace(/::$/, '');
  const st: DownloadState = reactive({ items: sel, base, format: 'zip', name: `${base}.zip` });
  win({
    title: t('share.download'), body: DownloadBody, props: { st },
    foot: [cancelBtn(), { label: t('share.download'), g: 'IconDownload', primary: true, run: () => {
      const file = st.name.trim() || `${base}.${st.format === 'zip' ? 'zip' : 'tar.gz'}`;
      tools.archiveStream(paths, st.format, file);
      toast(t('share.packing', { file }));
    } }],
  });
}

export default function setup() {
  // A link from the desktop app would lead to 127.0.0.1, which reaches nobody else.
  action('share', { label: tl('share.share'), g: 'IconLink', when: () => !host.desktop, run: shareWindow });
  action('collect', { label: tl('share.collect'), g: 'IconCollector', run: collect });
  action('collection', { label: () => (basket.length ? t('share.collectionCount', { n: basket.length }) : t('share.collection')), g: 'IconCollector', run: collectionWindow });
  action('download', { label: tl('share.download'), g: 'IconDownload', needs: 'sel', run: download });
  addTo('sideRows', () => (basket.length
    ? h(SidebarRow, { label: t('share.collection'), glyph: glyphComponent('IconCollector'), 'data-run': 'collection', onClick: () => run('collection') }, { after: () => h('span', { class: 'meta glim-num' }, String(basket.length)) })
    : null));
  const file = MENUS.find(([id]) => id === 'file')![1];
  if (!file.includes('collect')) file.splice(file.indexOf('share') + 1, 0, 'collect', 'collection');
  // The server tells every opening; a note only comes for links that asked for one.
  on('share.opened', (e) => {
    const { id, name } = e.data as { id: string; name: string };
    tools.getShare(id).then((s) => { if (s.notify) toast(t('share.opened', { name }), 'ok', { label: t('share.toShares'), run: () => run('openShares') }); }, () => {});
  });
}

