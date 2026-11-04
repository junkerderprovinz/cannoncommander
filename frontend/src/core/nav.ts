// Firefox keeps the mouse's side buttons for its own history and never shows them to the page (bug 1720538),
// so every folder change gets a history entry and stepping through the history steps through the folders.
// The active folder also stands in the hash, so a reload or a shared link opens it again.
import { run } from './actions';
import { view } from './views';
import { winStack } from './windows';

export interface NavSteps { el?: unknown; back: () => void; forward: () => void }
type Owner = number | NavSteps;

export const Nav = { at: 0, owner: [0] as Owner[], skip: false };

let hash = () => '';
let setActive = (_panel: number) => {};
export function setNavSource(source: { hash: () => string; setActive: (panel: number) => void }) {
  hash = source.hash;
  setActive = source.setActive;
}

const url = () => {
  const h = hash();
  return h ? '#' + encodeURI(h) : location.pathname + location.search;
};

export const hashPath = () => (location.hash.length > 1 ? decodeURI(location.hash.slice(1)) : '');

export function pushNav(owner: Owner) {
  Nav.at += 1;
  Nav.owner[Nav.at] = owner;
  Nav.owner.length = Nav.at + 1;
  history.pushState({ ccNav: Nav.at }, '', url());
}

// After a folder change the entry it pushed gets the new folder in its hash.
export const syncHash = () => history.replaceState(history.state, '', url());

// A window with its own folders takes part in the same history; closing it takes its steps back out.
export function winNav(w: unknown, steps: NavSteps) {
  const base = Nav.at;
  steps.el = w;
  return {
    push: () => pushNav(steps),
    close: () => {
      if (Nav.at > base) {
        Nav.skip = true;
        history.go(base - Nav.at);
      }
    },
  };
}

export function onPopState(e: PopStateEvent) {
  const to = (e.state as { ccNav?: number } | null)?.ccNav;
  if (to === undefined || to === Nav.at) return;
  const back = to < Nav.at, owner = Nav.owner[back ? Nav.at : to];
  Nav.at = to;
  if (Nav.skip) { Nav.skip = false; return; }
  if (owner && typeof owner === 'object') {
    const top = winStack.at(-1);
    if (top && (top === owner.el || top.el === owner.el)) (back ? owner.back : owner.forward)();
    return;
  }
  if (view.id !== 'files' || winStack.length) return;
  if (owner !== undefined) setActive(owner);
  run(back ? 'back' : 'forward');
}

export function installNav() {
  history.replaceState({ ccNav: 0 }, '', url());
  addEventListener('popstate', onPopState);
}
