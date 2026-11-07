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

function piece(cls: string, vars: Vars, html: string | Node = '', into: Element = host()) {
  const el = document.createElement('div');
  el.className = cls;
  for (const [k, v] of Object.entries(vars)) el.style.setProperty(k, typeof v === 'number' ? `${v}px` : v);
  if (typeof html === 'string') el.innerHTML = html;
  else el.append(html);
  const gone = (e: AnimationEvent) => { if (e.target === el) el.remove(); };
  el.addEventListener('animationend', gone);
  el.addEventListener('animationcancel', gone);
  into.append(el);
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

const puff = (at: Point, delay: string) =>
  [-1, 0, 1].map((side, i) => piece('fx-puff', { left: at.x, top: at.y, '--fx-side': String(side), '--fx-i': String(i), '--fx-delay': delay }, svg('IconCloud')));

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

// The smoke at the muzzle of the header's cannon as it fires.
export const muzzleSmoke = (at: Point) => puff(at, '0s');

// The logo's own ball, its shapes copied as they are and cut to their own box, flying to the screen position `to`.
// Flying left it is mirrored, so it rolls the other way.
export function flyBall(ball: SVGGraphicsElement, to: number) {
  const r = ball.getBoundingClientRect(), box = ball.getBBox(), dir = Math.sign(to - r.left) || 1;
  const drawing = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  drawing.setAttribute('viewBox', `${box.x} ${box.y} ${box.width} ${box.height}`);
  drawing.append(ball.cloneNode(true));
  return piece('fx-shot', { left: r.left, top: r.top, width: r.width, height: r.height, '--fx-dx': to - r.left, '--fx-flip': String(dir) }, drawing);
}

// Cuts a rectangle into rows of cells and every cell into two triangles, with the inner corners pushed about so no
// two shards match. Corners on the edge only slide along it, so the shards still cover the element exactly.
export function shards(cols: number, rows: number, rand = Math.random): [number, number][][] {
  const shake = (i: number, n: number) => (i > 0 && i < n ? (rand() - 0.5) * 0.7 : 0);
  const grid = Array.from({ length: cols + 1 }, (_, i) => Array.from({ length: rows + 1 }, (_, j): [number, number] =>
    [((i + shake(i, cols)) / cols) * 100, ((j + shake(j, rows)) / rows) * 100]));
  return grid.slice(0, -1).flatMap((col, i) => col.slice(0, -1).flatMap((a, j) => {
    const b = grid[i + 1]![j]!, c = grid[i + 1]![j + 1]!, d = col[j + 1]!;
    return rand() < 0.5 ? [[a, b, c], [a, c, d]] : [[a, b, d], [b, c, d]];
  }));
}

// A copy of the element that still finds its place in the head's styles: its parents come along as boxes that draw
// nothing, and the hooks that tests and handlers look for stay behind.
const HOOK = /^(id|tabindex|data-(run|go-view|account|all-menus|menu|tip))$/;
function lookalike(el: Element, head: Element, r: DOMRect) {
  const bare = (node: Element) => {
    for (const n of [node, ...node.querySelectorAll('*')]) for (const { name } of [...n.attributes]) if (HOOK.test(name)) n.removeAttribute(name);
    return node as HTMLElement;
  };
  const copy = bare(el.cloneNode(true) as Element);
  copy.classList.remove('fx-rebuild');
  Object.assign(copy.style, { position: 'absolute', left: '0', top: '0', width: `${r.width}px`, height: `${r.height}px`, margin: '0' });
  let top: Element = copy;
  for (let p = el.parentElement; p && p !== head; p = p.parentElement) {
    const shell = bare(p.cloneNode(false) as Element);
    shell.style.display = 'contents';
    shell.append(top);
    top = shell;
  }
  return top;
}

// Breaks an element of the head into shards that fly on the way the ball went, fall and fade. The element itself
// stays where it is; the caller hides it for as long as its shards are about.
export function shatter(el: Element, into: Element, dir: number, rand = Math.random) {
  const r = el.getBoundingClientRect();
  const cols = Math.min(5, Math.max(2, Math.round(r.width / 26)));
  return shards(cols, 2, rand).map((poly) => {
    const fall = document.createElement('div');
    fall.className = 'fx-shard-fall';
    fall.style.clipPath = `polygon(${poly.map(([x, y]) => `${x.toFixed(1)}% ${y.toFixed(1)}%`).join(', ')})`;
    fall.append(lookalike(el, into.parentElement!, r));
    return piece('fx-shard', {
      left: r.left, top: r.top, width: r.width, height: r.height,
      '--fx-sx': String(dir * (0.3 + rand())), '--fx-sy': String(0.5 + rand()), '--fx-kick': String(rand()), '--fx-sr': String(rand() * 2 - 1),
    }, fall, into);
  });
}
