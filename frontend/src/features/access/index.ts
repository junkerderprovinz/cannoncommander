// Screen readers: Alt+1 to Alt+4 jump between the regions, and the live region says what changed out of sight.
import { nextTick, watch } from 'vue';
import { action, run } from '@/core/actions';
import { say } from '@/core/announce';
import { fmtSize } from '@/core/format';
import { prefs, setPref } from '@/core/prefs';
import { go as goView, view } from '@/core/views';
import { addTo } from '@/core/extend';
import { jobLabel, placeName } from '@/features/jobs/label';
import { t, tl } from '@/i18n';
import { shell } from '@/shell/state';
import { useJobs } from '@/stores/jobs';
import { usePanels } from '@/stores/panels';

export default function setup() {
  const F = usePanels(), jobs = useJobs();
  const side = (i: number) => (prefs.dual ? t(i ? 'access.right' : 'access.left') : t('access.panel'));
  const where = (i = F.active) => {
    const n = F.entries(i).filter((k) => !k.up).length;
    return t('access.where', { side: side(i), folder: placeName(F.T(i).path), n, what: t(n === 1 ? 'access.one' : 'access.many') });
  };

  // Only a change is spoken: another folder or panel, or another number of marked entries.
  watch(() => {
    if (view.id !== 'files') return null;
    const tab = F.T(), marked = F.entries().filter((k) => tab.marked.has(k.name));
    return { panel: F.active, path: tab.path, n: marked.length, size: marked.reduce((a, k) => a + (k.size || 0), 0) };
  }, (now, last) => {
    if (!now || !last) return;
    if (now.path !== last.path || now.panel !== last.panel) say(where());
    else if (now.n !== last.n) say(now.n ? t('access.marked', { n: now.n, size: fmtSize(now.size) || '0 B' }) : t('access.unmarked'));
  });

  // A row with a toggle or a pin beside its link is pressed through that link.
  const pressOf = (row: HTMLElement) => row.querySelector<HTMLElement>(':scope > .glim-nav-row-main') ?? row;

  // The sidebar's rows and the lists say what they are; the arrows walk the rows below.
  function label() {
    const s = document.getElementById('side');
    if (!s) return;
    s.setAttribute('aria-label', t('access.sidebar'));
    s.querySelectorAll<HTMLElement>('.glim-nav-row').forEach((r) => { pressOf(r).tabIndex = -1; });
    document.querySelectorAll<HTMLElement>('.panel[data-panel] .list').forEach((l) => {
      const i = Number(l.closest<HTMLElement>('.panel')!.dataset.panel);
      l.setAttribute('aria-label', `${side(i)}: ${placeName(F.T(i).path)}`);
    });
  }
  addTo('onRender', label);

  jobs.onEnded((j) => {
    // After the marks a finished job clears have had their say, so the end of the job is what stays in the live region.
    if (j.state === 'done' || j.state === 'cancelled') void nextTick(() => say(t(j.state === 'done' ? 'access.done' : 'access.cancelled', { label: jobLabel(j) })));
  });

  async function toList(i: number) {
    if (view.id !== 'files') goView('files');
    if (i === 1 && !prefs.dual) setPref('dual', true);
    if (prefs.quickview && i !== F.active) setPref('quickview', false);
    F.active = i;
    await nextTick();
    label();
    document.querySelector<HTMLElement>(`.panel[data-panel="${i}"] .list`)?.focus();
  }
  async function toSide() {
    if (view.id !== 'files') goView('files');
    if (innerWidth <= 760) shell.sideOpen = true;
    else if (!prefs.side) setPref('side', true);
    await nextTick();
    label();
    const row = document.querySelector<HTMLElement>('#side .glim-nav-row.glim-active') || document.querySelector<HTMLElement>('#side .glim-nav-row');
    if (row) pressOf(row).focus();
    say(t('access.sideSay'));
  }
  function toJobs() {
    run('jobs');
    const n = jobs.running.length;
    setTimeout(() => (document.querySelector<HTMLElement>('#jobs-body [data-job].open') || document.querySelector<HTMLElement>('#jobs-mode button[aria-pressed="true"]'))?.focus(), 30);
    say(n ? t('access.jobsOpen', { n }) : t('access.jobsIdle'));
  }
  action('focusSide', { label: tl('access.focusSide'), g: 'IconMenu', key: 'Alt+1', run: toSide });
  action('focusLeft', { label: tl('access.focusLeft'), g: 'IconViewSimple', key: 'Alt+2', run: () => toList(0) });
  action('focusRight', { label: tl('access.focusRight'), g: 'IconCompare', key: 'Alt+3', run: () => toList(1) });
  action('focusJobs', { label: tl('access.focusJobs'), g: 'IconQueued', key: 'Alt+4', run: toJobs });

  // In the sidebar the arrows walk its rows; the frame's keys would move the list's cursor instead.
  document.addEventListener('keydown', (e) => {
    const row = (e.target as Element | null)?.closest?.<HTMLElement>('#side .glim-nav-row');
    if (!row || e.altKey || e.ctrlKey) return;
    const rows = [...document.querySelectorAll<HTMLElement>('#side .glim-nav-row')], i = rows.indexOf(row);
    const to = ({ ArrowDown: i + 1, ArrowUp: i - 1, Home: 0, End: rows.length - 1 } as Record<string, number>)[e.key];
    if (to !== undefined) pressOf(rows[Math.max(0, Math.min(rows.length - 1, to))]!).focus();
    else if (e.key === 'Enter' || e.key === ' ') pressOf(row).click();
    else if (e.key === 'Escape') { shell.sideOpen = false; document.querySelector<HTMLElement>('.panel.active .list')?.focus(); }
    else if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      const tog = row.querySelector<HTMLElement>('[data-tog]');
      if (tog && tog.classList.contains('open') !== (e.key === 'ArrowRight')) {
        const path = pressOf(row).dataset.go!;
        tog.click();
        void nextTick(() => document.querySelector<HTMLElement>(`#side .glim-nav-row-main[data-go="${CSS.escape(path)}"]`)?.focus());
      }
    } else return;
    e.preventDefault();
    e.stopImmediatePropagation();
  }, true);
}
