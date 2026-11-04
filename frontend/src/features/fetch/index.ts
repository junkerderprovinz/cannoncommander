// Von Adresse laden: the server fetches a file from a web address into a folder, as a job with progress.
import { reactive } from 'vue';
import type { PackResult } from '@/api/tools';
import { action } from '@/core/actions';
import { MENUS } from '@/core/menus';
import { toast } from '@/core/toast';
import { cancelBtn, win } from '@/core/windows';
import { split } from '@/files/paths';
import { kindLabels } from '@/features/jobs/label';
import { t, tl } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { clean, reveal, startJob } from '@/ui/tools';
import FetchBody, { type FetchState } from './FetchBody.vue';
import { parseUrl } from './url';

function fetchWindow() {
  const F = usePanels(), listing = useListing();
  const st: FetchState = reactive({ url: '', name: '', typed: false, dest: F.T().path });
  const focus = (el: HTMLElement, id: string) => el.querySelector<HTMLInputElement>(id)?.focus();
  win({
    title: t('fetch.title'), cls: 'bk', body: FetchBody, props: { st },
    foot: [{ start: t('ui.background') }, cancelBtn(), { label: t('fetch.load'), g: 'IconImport', primary: true, run: (el) => {
      const u = parseUrl(st.url), name = st.name.trim(), dest = clean(st.dest);
      if (!u) { toast(t('fetch.badUrl'), 'warn'); focus(el, '#fu-url'); return false; }
      if (!name || /[/\0]/.test(name)) { toast(t('fetch.badName'), 'warn'); focus(el, '#fu-name'); return false; }
      if (!listing.lists.has(dest) || dest.includes('::')) { toast(t('fetch.noDir'), 'warn'); return false; }
      void startJob('fetchUrl', { url: u.href, name, dest }, (j) => {
        const path = (j.result as PackResult).path, got = split(path).pop()!;
        toast(t('fetch.arrived', { name: got }), 'ok', { label: t('fetch.go'), run: () => { void F.go(dest).then(() => reveal(path)); } });
      }, { running: t('fetch.loading', { name }) });
      return true;
    } }],
  });
}

export default function setup() {
  action('fetchUrl', { label: tl('fetch.title'), g: 'IconImport', needs: 'writable', run: fetchWindow });
  kindLabels.fetchUrl = (_j, what) => t('fetch.job', { what });
  const file = MENUS.find(([id]) => id === 'file')![1];
  if (!file.includes('fetchUrl')) file.splice(file.indexOf('upload') + 1, 0, 'fetchUrl');
}
