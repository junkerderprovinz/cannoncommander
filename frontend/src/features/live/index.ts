// The list follows changes other programs make on the server: new files appear on their own and glow for a moment.
// While a window, a menu or a selection is open the refresh waits, so nothing moves under the pointer, and the cursor
// stays on the file it was on.
import './live.css';
import { watch } from 'vue';
import { writes } from '@/api/client';
import { isMenuOpen } from '@/core/menu';
import { prefs } from '@/core/prefs';
import { view } from '@/core/views';
import { winStack } from '@/core/windows';
import { fresh } from '@/files/fresh';
import { join } from '@/files/paths';
import { useJobs } from '@/stores/jobs';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';

const GLOW = 2500;
const OWN = 3000;

export default function setup() {
  const listing = useListing(), F = usePanels(), jobs = useJobs();
  const pending = new Set<string>();
  const panels = () => (prefs.dual ? [0, 1] : [0]);
  const busy = () => view.id !== 'files' || !!winStack.length || isMenuOpen() || !!document.querySelector('.qdrop, .cpick, .a-cpop')
    || panels().some((i) => F.T(i).marked.size) || !!document.activeElement?.matches('input, textarea');

  function refresh(path: string, glow = true) {
    pending.delete(path);
    const before = new Map((listing.lists.get(path)?.items ?? []).map((k) => [k.name, `${k.size}:${k.time}`]));
    const at = panels().map((i) => (F.T(i).path === path ? F.cursorEntry(i)?.name : undefined));
    listing.changed(path);
    const stop = watch(() => listing.lists.get(path), (got) => {
      if (!got || got.items.length === before.size && got.items.every((k) => before.get(k.name) === `${k.size}:${k.time}`)) return;
      stop();
      const now = got.items.filter((k) => before.get(k.name) !== `${k.size}:${k.time}`).map((k) => join(path, k.name));
      if (glow) {
        now.forEach((p) => fresh.add(p));
        setTimeout(() => now.forEach((p) => fresh.delete(p)), GLOW);
      }
      panels().forEach((i, n) => {
        const k = at[n] ? F.entries(i).findIndex((x) => x.name === at[n]) : -1;
        if (k >= 0) F.T(i).cursor = k;
      });
    });
    setTimeout(stop, 10_000);
  }

  // A change this page caused, by a step it asked for a moment ago or a job of its own that ended, shows at once and
  // without the glow. A job tells the folders it changed before it says it is done, so its end hands those on too.
  const own = () => Date.now() - writes.at < OWN;
  listing.setGate((path) => {
    if (own()) return true;
    if (prefs.liveRefresh === false || busy()) pending.add(path);
    else refresh(path);
    return false;
  });
  setInterval(() => { if (prefs.liveRefresh !== false && !busy()) [...pending].forEach((p) => refresh(p)); }, 700);
  jobs.onEnded(() => [...pending].forEach((p) => refresh(p, false)));
}
