// The step files find things by the prototype's class names and attributes, and GlimStone's components draw their own.
// In the test build this names GlimStone's markup the way the step files expect, so production carries only the real one.
const CLASSES: [selector: string, name: string][] = [
  ['.glim-modal-backdrop', 'scrim'],
  ['.glim-window-title .glim-badge, .glim-notch-card > h2 .glim-badge', 'notch'],
  ['.glim-notch-card', 'card'],
  ['.glim-window-body', 'win-body'],
  ['.glim-window .glim-modal-foot', 'win-foot'],
  ['.glim-window-start', 'start'],
  ['.glim-window .glim-modal-foot button', 'btn'],
  ['.glim-window .glim-modal-foot .glim-btn--accent', 'primary'],
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

function name() {
  for (const [selector, cls] of CLASSES) {
    document.querySelectorAll(selector).forEach((el) => { if (!el.classList.contains(cls)) el.classList.add(cls); });
  }
  document.querySelectorAll<HTMLElement>('.glim-window .glim-modal-foot').forEach((foot) => {
    foot.querySelectorAll<HTMLElement>('.glim-window-start, button').forEach((el, k) => {
      if (el.tagName === 'BUTTON' && el.dataset.f !== String(k)) el.dataset.f = String(k);
    });
  });
  document.querySelectorAll<HTMLElement>('.glim-window').forEach((card) => {
    const title = card.querySelector('.glim-window-title')?.textContent ?? '';
    if (card.getAttribute('aria-label') !== title) card.setAttribute('aria-label', title);
  });
  // Card follows its title with a space for the hint, which a step file comparing the title would trip over.
  document.querySelectorAll('.glim-notch-card > h2 .glim-badge').forEach((badge) => {
    if (badge.querySelector('.glim-info-icon')) return;
    badge.childNodes.forEach((n) => { if (n.nodeType === Node.TEXT_NODE && n.nodeValue !== n.nodeValue!.trimEnd()) n.nodeValue = n.nodeValue!.trimEnd(); });
  });
  document.querySelectorAll<HTMLElement>('[data-sel-id]').forEach((seg) => {
    if (seg.dataset.v !== seg.dataset.selId) seg.dataset.v = seg.dataset.selId;
    if (seg.closest('.head') && seg.dataset.run !== seg.dataset.selId) seg.dataset.run = seg.dataset.selId;
    const on = seg.getAttribute('aria-checked') ?? seg.getAttribute('aria-selected') ?? 'false';
    if (seg.getAttribute('aria-pressed') !== on) seg.setAttribute('aria-pressed', on);
  });
}

export function installHooks() {
  name();
  new MutationObserver(name).observe(document.documentElement, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: ['class', 'aria-checked', 'aria-selected'] });
}
