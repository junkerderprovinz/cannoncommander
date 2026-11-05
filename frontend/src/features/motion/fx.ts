// Motion that plays over the page rather than inside one component: a layer of its own above the panels, with
// nothing in it that takes a click. Every piece reads its numbers from the level's dials in motion.css and takes
// itself away when its animation ends, so a quick level and a slow one leave the page the same.
import { glyph } from '@/core/glyphs';

type Vars = Record<string, string | number>;
interface Point { x: number; y: number }

const centre = (el: Element): Point => {
  const r = el.getBoundingClientRect();
  return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
};

let layer: HTMLElement | null = null;
function host() {
  if (!layer?.isConnected) {
    layer = document.createElement('div');
    layer.className = 'fx-layer';
    layer.setAttribute('aria-hidden', 'true');
    document.body.append(layer);
  }
  return layer;
}

const svg = (name: string) => {
  const g = glyph(name);
  return `<svg viewBox="${g.viewBox}" fill="currentColor">${g.svg}</svg>`;
};

// A tumbleweed and a cannonball have no glyph in GlimStone, so they are drawn here in its manner: filled shapes on
// the 14 unit grid, inked by the colour they stand in. The tumbleweed is a ring around three crossed loops of twig.
const TWIG = 'M1.8 7a5.2 2.5 0 1 0 10.4 0a5.2 2.5 0 1 0-10.4 0ZM3 7a4 1.35 0 1 0 8 0a4 1.35 0 1 0-8 0Z';
const WEED = `<svg viewBox="0 0 14 14" fill="currentColor" fill-rule="evenodd"><path d="M0.5 7a6.5 6.5 0 1 0 13 0a6.5 6.5 0 1 0-13 0ZM1.7 7a5.3 5.3 0 1 0 10.6 0a5.3 5.3 0 1 0-10.6 0Z"/>${
  [0, 60, 120].map((a) => `<path d="${TWIG}" transform="rotate(${a} 7 7)"/>`).join('')}</svg>`;
const BALL = `<svg viewBox="0 0 14 14"><circle cx="7" cy="7" r="6" fill="currentColor"/><circle class="fx-shine" cx="4.9" cy="4.9" r="1.5"/></svg>`;

function piece(cls: string, vars: Vars, html = '') {
  const el = document.createElement('div');
  el.className = cls;
  for (const [k, v] of Object.entries(vars)) el.style.setProperty(k, typeof v === 'number' ? `${v}px` : v);
  el.innerHTML = html;
  const gone = (e: AnimationEvent) => { if (e.target === el) el.remove(); };
  el.addEventListener('animationend', gone);
  el.addEventListener('animationcancel', gone);
  host().append(el);
  return el;
}

// Plays a class on an element of the page once, again from the start when it is already playing.
export function replay(el: Element, cls: string) {
  el.classList.remove(cls);
  void (el as HTMLElement).offsetWidth;
  el.classList.add(cls);
  const done = (e: Event) => {
    if (e.target !== el) return;
    el.classList.remove(cls);
    el.removeEventListener('animationend', done);
  };
  el.addEventListener('animationend', done);
}

export function trace(from: Element, to: Element) {
  const a = centre(from), b = centre(to);
  piece('fx-trace', { left: b.x, top: b.y, '--fx-dx': b.x - a.x, '--fx-dy': b.y - a.y, '--fx-angle': `${Math.atan2(b.y - a.y, b.x - a.x)}rad` });
}

const puff = (at: Point, delay: string) => {
  [-1, 0, 1].forEach((side, i) => piece('fx-puff', { left: at.x, top: at.y, '--fx-side': String(side), '--fx-i': String(i), '--fx-delay': delay }, svg('IconCloud')));
};

export function cannon(from: Element, to: Element) {
  const a = centre(from), b = centre(to);
  piece('fx-cannon', { left: b.x, top: b.y, '--fx-dx': b.x - a.x, '--fx-dy': b.y - a.y }, `<div class="fx-ball">${BALL}</div>`);
  puff(b, 'var(--motion-cannon-dur)');
}

// Rows on their way to the trash, at most a handful: a hundred falling entries would only hide what is left. With no
// trash in sight they fall toward the bottom edge.
export function drop(rows: { el: Element; glyph: string; name: string }[], bin?: Element | null) {
  const target = bin && centre(bin);
  rows.slice(0, 8).forEach(({ el, glyph: g, name }, i) => {
    const r = el.getBoundingClientRect();
    const label = document.createElement('span');
    label.textContent = name;
    piece('fx-drop', { left: r.left, top: r.top, '--fx-dx': target ? target.x - r.left : 0, '--fx-dy': target ? target.y - r.top : 160, '--fx-i': String(i) }, svg(g) + label.outerHTML);
  });
}

export function smoke(from: Element) {
  const r = from.getBoundingClientRect();
  [0, 1, 2].forEach((i) => piece('fx-smoke', { left: r.left + r.width / 2, top: r.top, '--fx-i': String(i) }, svg('IconCloud')));
}

export function salute(bar: Element) {
  const r = bar.getBoundingClientRect(), n = 7;
  for (let i = 0; i < n; i++) piece('fx-salute', { left: r.left + (r.width * (i + 0.5)) / n, top: r.top + r.height / 2, '--fx-i': String(i) }, svg('IconStar'));
}

export function tumble(box: Element) {
  const r = box.getBoundingClientRect();
  return piece('fx-tumble', { left: r.left, top: r.top, width: r.width, height: r.height }, `<div class="fx-tumble-run"><div class="fx-weed">${WEED}</div></div>`);
}
