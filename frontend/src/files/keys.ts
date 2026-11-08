// Keys of the file view itself, after windows, menus and the key map had their turn.
import { reactive } from 'vue';
import { run } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { usePanels } from '@/stores/panels';
import { CARD_VIEWS } from './views';

// Typing in the list jumps to the first name that starts with the letters typed so far, as in Krusader.
export const jump = reactive({ text: '', found: true });
let jumpTimer: ReturnType<typeof setTimeout> | undefined;
export function jumpEnd() {
  jump.text = '';
  clearTimeout(jumpTimer);
}
const fold = (s: string) => s.toLocaleLowerCase('de');
const rearm = () => {
  clearTimeout(jumpTimer);
  jumpTimer = setTimeout(jumpEnd, 4000);
};

export function jumpTo(from: number, dir: number) {
  const F = usePanels();
  const list = F.entries(), n = list.length;
  for (let s = 0; s < n; s++) {
    const i = (((from + s * dir) % n) + n) % n;
    if (!list[i]!.up && fold(list[i]!.name).startsWith(fold(jump.text))) {
      F.T().cursor = i;
      return (jump.found = true);
    }
  }
  return (jump.found = false);
}

// Cards wrap into rows and the list view into columns, so one arrow pair steps by a whole row or column there.
function across(flow: boolean) {
  const rows = [...document.querySelectorAll<HTMLElement>('.panel.active .list > .row')];
  if (!rows.length) return 1;
  return Math.max(1, rows.filter((r) => (flow ? r.offsetLeft === rows[0]!.offsetLeft : r.offsetTop === rows[0]!.offsetTop)).length);
}

export function filesKey(e: KeyboardEvent, combo: string) {
  const F = usePanels();
  const typed = e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey && (e.key !== ' ' || !!jump.text);
  if (typed) {
    e.preventDefault();
    jump.text += e.key;
    rearm();
    jumpTo(F.T().cursor, 1);
    return;
  }
  if (jump.text) {
    if (combo === 'Escape') { e.preventDefault(); return jumpEnd(); }
    if (combo === 'Backspace') {
      e.preventDefault();
      jump.text = jump.text.slice(0, -1);
      if (!jump.text) return jumpEnd();
      jumpTo(0, 1);
      return;
    }
    if (combo === 'ArrowDown' || combo === 'ArrowUp') {
      e.preventDefault();
      rearm();
      const dir = combo === 'ArrowDown' ? 1 : -1;
      jumpTo(F.T().cursor + dir, dir);
      return;
    }
    jumpEnd();
  }
  plainKey(e, combo);
}

function plainKey(e: KeyboardEvent, combo: string) {
  const F = usePanels();
  const t = F.T(), list = F.entries(), view = F.viewOf(t);
  const move = (to: number) => { t.cursor = Math.max(0, Math.min(list.length - 1, to)); };
  const grid = CARD_VIEWS.includes(view), flow = view === 'list';
  const markStep = () => {
    const k = list[t.cursor];
    if (k && !k.up) t.marked.has(k.name) ? t.marked.delete(k.name) : t.marked.add(k.name);
    move(t.cursor + 1);
  };
  // As in Explorer, the marks follow the cursor, and Shift takes the entries it passes along.
  const take = () => {
    const k = list[t.cursor];
    if (k && !k.up) t.marked.add(k.name);
  };
  const keys: Record<string, () => unknown> = {
    Tab: () => { if (prefs.dual && !prefs.quickview) F.active = F.other(); },
    ArrowDown: () => move(t.cursor + (grid ? across(false) : 1)), ArrowUp: () => move(t.cursor - (grid ? across(false) : 1)),
    Home: () => move(0), End: () => move(list.length - 1),
    ...(grid || flow ? { ArrowRight: () => move(t.cursor + (flow ? across(true) : 1)), ArrowLeft: () => move(t.cursor - (flow ? across(true) : 1)) } : {}),
    ...(view === 'columns' ? {
      ArrowLeft: () => t.path !== '/' && F.open(F.active, { up: true }),
      ArrowRight: () => list[t.cursor]?.dir && F.open(F.active, list[t.cursor]!),
    } : {}),
    PageDown: () => move(t.cursor + 10), PageUp: () => move(t.cursor - 10),
    Enter: () => list[t.cursor] && F.open(F.active, list[t.cursor]!),
    Backspace: () => t.path !== '/' && F.open(F.active, { up: true }),
    Insert: markStep, ' ': () => run('quickLook'),
    ...(prefs.clickSelect ? {
      'Shift+ArrowDown': () => { take(); move(t.cursor + 1); take(); },
      'Shift+ArrowUp': () => { take(); move(t.cursor - 1); take(); },
    } : {}),
    Escape: () => {
      if (t.qf || t.filter) Object.assign(t, { qf: false, filter: '', cursor: 0 });
      else if (Object.keys(F.marks[0]!).length || Object.keys(F.marks[1]!).length) F.marks.splice(0, 2, {}, {});
      else if (t.marked.size) t.marked = new Set();
    },
  };
  if (keys[combo]) {
    e.preventDefault();
    keys[combo]!();
    if (prefs.clickSelect && STEPS.includes(combo)) {
      const k = F.entries()[F.T().cursor];
      F.T().marked = k && !k.up ? new Set([k.name]) : new Set();
    }
  }
}

const STEPS = ['ArrowDown', 'ArrowUp', 'ArrowLeft', 'ArrowRight', 'Home', 'End', 'PageDown', 'PageUp'];

// While letters are typed to jump, Backspace and the arrows belong to the typing, also where the key map gives them
// to an action, as Backspace in Explorer's layout.
export const typingClaims = (combo: string) => !!jump.text && ['Backspace', 'Escape', 'ArrowUp', 'ArrowDown'].includes(combo);

export const installJumpReset = () => document.addEventListener('mousedown', () => { if (jump.text) jumpEnd(); });
