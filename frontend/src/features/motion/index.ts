// Motion for the moments a file manager has: a copy or move setting off, entries going to the trash, a job coming
// home. And a few surprises that only turn up for somebody looking for them. Nothing here waits for an animation,
// so none of it can hold up the work it decorates.
import { nextTick, watch } from 'vue';
import type { Job, JobCreate } from '@/api/ops';
import { addTo } from '@/core/extend';
import { glyphOf } from '@/core/glyphs';
import { levelOf, lively, moving, stormy } from '@/core/motion';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { join } from '@/files/paths';
import { plural, t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { usePanels } from '@/stores/panels';
import { useTrash } from '@/stores/trash';
import { broadside, KONAMI, onceEach, sequence, typedWord } from './cues';
import { cannon, drop, replay, salute, smoke, trace, tumble } from './fx';
import './motion.css';

type Params = { sources?: string[]; dest?: string; paths?: string[] };

// Boxes rather than offsetParent, which an svg does not have.
const shown = (el: Element | null | undefined): el is Element => !!el && el.getClientRects().length > 0;
const panelEl = (i: number) => document.querySelector<HTMLElement>(`.panel[data-panel="${i}"] .list`);
const jobsButton = () => document.querySelector('.head [data-run="jobs"]');
const panelsShown = () => (prefs.dual ? [0, 1] : [0]);

// The cannon answers the copy command itself, F5 or its menu entry, not a copy dragged across.
let copyCommand = false;

function onStarted(_job: Job, req: JobCreate) {
  if (!moving()) return;
  const F = usePanels(), p = req.params as Params;
  if ((req.kind === 'copy' || req.kind === 'move') && p.sources?.length) {
    const from = p.sources[0]!.slice(0, p.sources[0]!.lastIndexOf('/')) || '/';
    const at = (path?: string) => panelsShown().map(panelEl).find((el, i) => shown(el) && F.T(i).path === path) ?? null;
    const source = at(from) ?? panelEl(F.active), target = at(p.dest) ?? jobsButton();
    if (shown(source) && shown(target)) {
      if (req.kind === 'copy' && copyCommand && stormy()) cannon(source, target);
      else trace(source, target);
    }
    if (req.kind === 'copy' && broadside(levelOf(), p.sources.length)) toast(plural(p.sources.length, 'motion.broadside'));
  }
  if ((req.kind === 'trash' || req.kind === 'delete') && p.paths?.length) {
    const gone = new Set(p.paths);
    const rows = panelsShown().flatMap((i) => [...document.querySelectorAll(`.panel[data-panel="${i}"] .row[data-idx]`)].flatMap((el) => {
      const k = F.entries(i)[+(el as HTMLElement).dataset.idx!];
      return k && !k.up && gone.has(join(k.at || F.T(i).path, k.name)) ? [{ el, glyph: glyphOf(k), name: k.name }] : [];
    }));
    const bin = [document.querySelector('#side [data-run="openTrash"] svg'), document.querySelector('#fkeys [data-run="delete"]')].find(shown);
    drop(rows, bin);
  }
  copyCommand = false;
}

function onEnded(job: Job) {
  const button = jobsButton();
  if (!moving() || !shown(button)) return;
  if (job.state === 'done') replay(button, 'glim-confirm');
  else if (job.state === 'failed') replay(button, 'glim-shake');
}

// The smoke waits for both the job's end and the empty list, which arrive in either order.
function smokeAfterEmptying() {
  const trash = useTrash();
  let emptied = false;
  const puff = () => {
    if (!emptied || trash.items.length) return;
    emptied = false;
    void nextTick(() => {
      const bin = document.querySelector('.ct-empty svg');
      if (moving() && shown(bin)) smoke(bin);
    });
  };
  useJobs().onEnded((job) => {
    if (job.kind !== 'emptyTrash' || job.state !== 'done') return;
    emptied = true;
    puff();
  });
  watch(() => trash.items.length, puff);
}

function konami() {
  const next = sequence(KONAMI);
  addEventListener('keydown', (e) => {
    if ((e.target as Element | null)?.closest?.('input, textarea, select, [contenteditable]') || !next(e.key)) return;
    const head = document.querySelector('.head');
    if (!moving() || !head) return;
    salute(head);
    toast(t('motion.salute'));
  }, true);
}

function kaboom() {
  const F = usePanels(), word = typedWord('kaboom');
  watch(() => [F.T(), F.T().filter ?? ''] as const, ([tab, text], [was, before]) => {
    const head = document.querySelector('.head');
    if (tab === was && word(before, text) && moving() && head) replay(head, 'fx-recoil');
  });
}

// One roll per visit: a visit is a folder entered, so a redraw, a sort or a refresh of the same folder plays nothing.
function tumbleweed() {
  const F = usePanels(), take = onceEach(), visits = [0, 0], rolling: (HTMLElement | undefined)[] = [];
  addTo('afterGo', (_path, i) => { visits[i] = (visits[i] ?? 0) + 1; });
  watch(() => panelsShown().map((i) => (F.loaded(i) && !F.entries(i).length && !F.T(i).filter ? `${i}:${visits[i]}:${F.T(i).path}` : '')), (keys) => {
    keys.forEach((key, i) => {
      // A folder that fills up while it rolls has no room for a tumbleweed any more.
      if (!key) rolling[i]?.remove();
      if (!key || !lively() || !take(key)) return;
      void nextTick(() => {
        const list = panelEl(i);
        if (shown(list)) rolling[i] = tumble(list);
      });
    });
  });
}

export default function setup() {
  const jobs = useJobs();
  jobs.onStarted(onStarted);
  jobs.onEnded(onEnded);
  addTo('actionWrappers', (a, run) => (...args: never[]) => {
    if (a.id === 'copy') copyCommand = true;
    else if (a.id !== 'transfer') copyCommand = false;
    return run(...args);
  });
  smokeAfterEmptying();
  konami();
  kaboom();
  tumbleweed();
}
