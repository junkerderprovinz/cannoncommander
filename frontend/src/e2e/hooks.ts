// The step files find things by the prototype's class names and attributes, and GlimStone's components draw their own.
// In the test build this names GlimStone's markup the way the step files expect, so production carries only the real one.
const CLASSES: [selector: string, name: string][] = [
  ['.glim-modal-backdrop', 'scrim'],
  ['.glim-window-title .glim-badge, .glim-notch-card > h2 .glim-badge', 'notch'],
  ['.glim-notch-card', 'card'],
  ['.glim-window-body', 'win-body'],
  ['.glim-window .glim-modal-foot', 'win-foot'],
  ['.glim-window-start', 'start'],
  ['.glim-btn', 'btn'],
  ['.glim-btn--accent', 'primary'],
  ['.glim-btn-key', 'key'],
  ['.glim-icon-btn', 'btn'],
  ['.glim-icon-btn', 'icon'],
  ['.glim-field-row', 'frow'],
  ['.glim-field-row-name', 'lab'],
  ['.glim-list-row', 'a-row'],
  ['.glim-list-row-title', 't'],
  ['.glim-sel', 'seg'],
  ['.glim-kbd', 'kbd'],
  ['.glim-table', 'tbl'],
  ['.glim-nav-row', 'nav-row'],
  ['.glim-nav-row.glim-active', 'cur'],
  ['.glim-subcard', 'well'],
  ['.glim-toast', 'toast'],
  ['.glim-pill', 'pill'],
  ['.glim-pill--state', 'state'],
  ['.glim-pill--ok', 'ok'],
  ['.glim-pill--warn', 'warn'],
  ['.glim-pill--fail', 'fault'],
  ['.row .name .glim-pill:is(.glim-pill--ok, .glim-pill--warn, .glim-pill--fail)', 'state'],
  ['.glim-picker-panel', 'a-cpop'],
  ['.glim-picker-panel', 'cpick'],
  ['.glim-picker-sv', 'cp-sv'],
  ['.glim-picker-hue', 'cp-hue'],
  ['.glim-picker-hex', 'cp-hex'],
  ['.glim-picker-name', 'cp-name'],
  ['.glim-picker-actions button', 'cp-ok'],
  ['[role=switch].glim-toggle-track', 'sw'],
  ['.bk-perms .glim-checkbox-box', 'cbox'],
];

function closedLists() {
  let holder = document.getElementById('e2e-lists');
  if (!holder) {
    holder = Object.assign(document.createElement('div'), { id: 'e2e-lists' });
    document.body.prepend(holder);
  }
  return holder;
}

function nameSelects() {
  // A select keeps its closed list on the body, where the steps' .scrim:last-of-type expects the windows last. An open
  // list goes back to the end, to lie over its window.
  document.querySelectorAll<HTMLElement>('.glim-select-positioner').forEach((list) => {
    const open = !!list.querySelector('[data-state=open]');
    if (open && list.parentElement !== document.body) document.body.append(list);
    if (!open && list.parentElement === document.body) closedLists().append(list);
  });
  document.querySelectorAll<HTMLElement>('.glim-select-content').forEach((list) => {
    const group = list.querySelector<HTMLElement>('.glim-select-group');
    if (group) group.id = list.dataset.state === 'open' ? 'menu' : '';
    list.querySelectorAll<HTMLElement>('[role=option]').forEach((row) => {
      const on = row.dataset.state === 'checked';
      if (row.classList.contains('on') !== on) row.classList.toggle('on', on);
    });
  });
}

function name() {
  for (const [selector, cls] of CLASSES) {
    document.querySelectorAll(selector).forEach((el) => { if (!el.classList.contains(cls)) el.classList.add(cls); });
  }
  document.querySelectorAll<HTMLElement>('.glim-window').forEach((card) => {
    const title = card.querySelector('.glim-window-title')?.textContent ?? '';
    if (card.getAttribute('aria-label') !== title) card.setAttribute('aria-label', title);
  });
  document.querySelectorAll<HTMLElement>('[data-sel-id]').forEach((seg) => {
    if (seg.dataset.v !== seg.dataset.selId) seg.dataset.v = seg.dataset.selId;
    const on = seg.getAttribute('aria-checked') ?? seg.getAttribute('aria-selected') ?? 'false';
    if (seg.getAttribute('aria-pressed') !== on) seg.setAttribute('aria-pressed', on);
  });
  // The steps read a row's folder and its action from the row around its link.
  document.querySelectorAll<HTMLElement>('.glim-nav-row--slots').forEach((row) => {
    const main = row.querySelector<HTMLElement>(':scope > .glim-nav-row-main')!;
    for (const key of ['go', 'run', 'parity']) if (main.dataset[key] !== undefined && row.dataset[key] !== main.dataset[key]) row.dataset[key] = main.dataset[key];
  });
  nameSelects();
}

export function installHooks() {
  name();
  new MutationObserver(name).observe(document.documentElement, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: ['class', 'aria-checked', 'aria-selected', 'data-state'] });
}
