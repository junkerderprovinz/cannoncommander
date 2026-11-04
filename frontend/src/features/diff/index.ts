// Dateien vergleichen: two marked files, or the file under the cursor in each panel, line by line.
import { tools } from '@/api/tools';
import { action } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { join } from '@/files/paths';
import { t, tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import DiffBody from './DiffBody.vue';
import './diff.css';

function pickPair(): [string, string] | null {
  const F = usePanels(), here = F.T().path;
  const sel = F.sel().filter((k) => !k.dir);
  if (sel.length === 2) return [join(sel[0]!.at || here, sel[0]!.name), join(sel[1]!.at || here, sel[1]!.name)];
  if (prefs.dual) {
    const a = F.cursorEntry(0), b = F.cursorEntry(1);
    if (a && b && !a.dir && !b.dir && !a.up && !b.up) return [join(a.at || F.T(0).path, a.name), join(b.at || F.T(1).path, b.name)];
  }
  return null;
}

export default function setup() {
  action('diff', { label: tl('diff.title'), g: 'IconCompare', run: async () => {
    const pair = pickPair();
    if (!pair) return toast(prefs.dual ? t('diff.pickTwoDual') : t('diff.pickTwo'), 'warn');
    try {
      const diff = await tools.diff(...pair);
      win({ title: t('diff.title'), full: true, cls: 'bk bk-diff', body: DiffBody, props: { diff } });
    } catch (e) {
      toast((e as Error).message, 'warn');
    }
  } });
}
