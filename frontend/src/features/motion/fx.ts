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

// The burst in front of the muzzle as the header's cannon fires, pointing the way the ball goes.
export const muzzleFlash = (at: Point, size: number, dir: number) =>
  piece('fx-flash', { left: at.x, top: at.y, '--fx-size': size, '--fx-turn': dir > 0 ? '0deg' : '180deg' });

// The cloud the shot leaves: soft puffs that start at the muzzle, swell, drift on after the ball and rise.
export function gunSmoke(at: Point, size: number, dir: number, rand = Math.random) {
  return [0, 1, 2, 3, 4].map((i) => piece('fx-gunsmoke', {
    left: at.x + dir * size * (0.2 + i * 0.3), top: at.y + (rand() - 0.5) * size * 0.4, '--fx-size': size * (0.8 + rand() * 0.5),
    '--fx-gx': String(dir * (0.4 + i * 0.25 + rand() * 0.4)), '--fx-gy': String(0.3 + rand() * 0.7), '--fx-i': String(i),
  }));
}

// The logo's own ball, its shapes copied as they are and cut to their own box, flying from the screen position `from`
// to `to` with a trail of fire behind it. Flying left it is mirrored, so it rolls the other way.
export function flyBall(ball: SVGGraphicsElement, from: number, to: number) {
  const r = ball.getBoundingClientRect(), box = ball.getBBox(), dir = Math.sign(to - from) || 1;
  const drawing = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  drawing.setAttribute('viewBox', `${box.x} ${box.y} ${box.width} ${box.height}`);
  drawing.append(ball.cloneNode(true));
  const shot = piece('fx-shot', { left: from, top: r.top, width: r.width, height: r.height, '--fx-dx': to - from, '--fx-flip': String(dir) }, drawing);
  const trail = document.createElement('div');
  trail.className = 'fx-trail';
  shot.prepend(trail);
  return shot;
}

export interface Shard {
  /** The corners, in percent of the box. */
  poly: [number, number][];
  /** The middle of the corners, in percent of the box. */
  centre: [number, number];
  /** One unit long, from the strike towards the shard. */
  out: [number, number];
  /** How far from the strike, from 0 to 1 for the farthest shard. */
  dist: number;
}

// Cracks a w by h box the way a pane breaks where a stone hits it: rays run from the strike to the edge, two jagged
// rings cross them, and every cell between them is a shard. The cells fill the box exactly, so the pieces still look
// like the element in the moment it breaks. The strike moves a little inside, where rays can leave it on every side.
export function shards(w: number, h: number, at: Point, rand = Math.random): Shard[] {
  const ix = Math.min(8, w * 0.12), iy = Math.min(8, h * 0.12);
  const P = { x: Math.min(w - ix, Math.max(ix, at.x)), y: Math.min(h - iy, Math.max(iy, at.y)) };
  const n = Math.max(8, Math.min(14, Math.round((w + h) / 12))), step = (2 * Math.PI) / n, turn = rand() * step;
  const angles = Array.from({ length: n }, (_, i) => turn + i * step + (rand() - 0.5) * step * 0.6);
  const rays = angles.map((a) => {
    const ux = Math.cos(a), uy = Math.sin(a);
    const reach = Math.min(ux > 0 ? (w - P.x) / ux : ux < 0 ? -P.x / ux : Infinity, uy > 0 ? (h - P.y) / uy : uy < 0 ? -P.y / uy : Infinity);
    return [0.22 + rand() * 0.2, 0.55 + rand() * 0.22, 1].map((f): [number, number] => [P.x + ux * reach * f, P.y + uy * reach * f]);
  });
  const corners: [number, number][] = [[0, 0], [w, 0], [w, h], [0, h]];
  const past = (from: number, a: number) => (((a - from) % (2 * Math.PI)) + 2 * Math.PI) % (2 * Math.PI);
  const cells = rays.flatMap((ray, i) => {
    const next = rays[(i + 1) % n]!, from = angles[i]!, gap = past(from, angles[(i + 1) % n]!);
    const between = corners.map((c) => ({ c, a: past(from, Math.atan2(c[1] - P.y, c[0] - P.x)) }))
      .filter(({ a }) => a > 0 && a < gap).sort((x, y) => x.a - y.a).map(({ c }) => c);
    return [
      [[P.x, P.y], ray[0]!, next[0]!],
      [ray[0]!, ray[1]!, next[1]!, next[0]!],
      [ray[1]!, ray[2]!, ...between, next[2]!, next[1]!],
    ] as [number, number][][];
  });
  const middles = cells.map((c) => [c.reduce((s, p) => s + p[0], 0) / c.length, c.reduce((s, p) => s + p[1], 0) / c.length] as const);
  const far = Math.max(...middles.map(([x, y]) => Math.hypot(x - P.x, y - P.y)));
  return cells.map((c, k) => {
    const [x, y] = middles[k]!, d = Math.hypot(x - P.x, y - P.y) || 1;
    return {
      poly: c.map(([px, py]): [number, number] => [(px / w) * 100, (py / h) * 100]),
      centre: [(x / w) * 100, (y / h) * 100],
      out: [(x - P.x) / d, (y - P.y) / d],
      dist: d / far,
    };
  });
}
const HOOK = /^(id|tabindex|data-(run|go-view|account|all-menus|menu|tip))$/;
const bare = (node: Element) => {
  for (const n of [node, ...node.querySelectorAll('*')]) for (const { name } of [...n.attributes]) if (HOOK.test(name)) n.removeAttribute(name);
  return node as HTMLElement;
};

// The shards fly over the whole page, above everything on it. They lie inside a copy of the head that draws no box
// of its own, so the copies they are cut from still take the head's styles.
export function shardLayer(head: Element) {
  const layer = document.createElement('div');
  layer.className = 'fx-shards';
  layer.setAttribute('aria-hidden', 'true');
  layer.inert = true;
  const frame = bare(head.cloneNode(false) as Element);
  frame.style.display = 'contents';
  layer.append(frame);
  document.body.append(layer);
  return { layer, frame };
}

// A copy of the element that still finds its place in the head's styles: its parents come along as boxes that draw
// nothing, and the hooks that tests and handlers look for stay behind.
function lookalike(el: Element, head: Element, r: DOMRect) {
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

// Breaks an element of the head where the ball strikes it, at the height `y` on the side it comes from: a short flash
// and a puff of dust there, and the element bursts into shards that keep the ball's momentum, each a little more or
// less of it, and fly away from the strike as well, turn and fall. The element itself stays where it is; the caller
// hides it while its shards are about.
export function shatter(el: Element, head: Element, into: Element, dir: number, y: number, rand = Math.random) {
  const r = el.getBoundingClientRect();
  const strike = { x: dir > 0 ? r.left : r.right, y: Math.min(r.bottom, Math.max(r.top, y)) };
  const size = Math.max(14, Math.min(r.height, 40) * 0.7);
  const bits = shards(r.width, r.height, { x: strike.x - r.left, y: strike.y - r.top }, rand).map((s) => {
    const fall = document.createElement('div');
    fall.className = 'fx-shard-fall';
    fall.style.clipPath = `polygon(${s.poly.map(([px, py]) => `${px.toFixed(1)}% ${py.toFixed(1)}%`).join(', ')})`;
    fall.style.transformOrigin = `${s.centre[0].toFixed(1)}% ${s.centre[1].toFixed(1)}%`;
    fall.append(lookalike(el, head, r));
    const [ux, uy] = s.out;
    return piece('fx-shard', {
      left: r.left, top: r.top, width: r.width, height: r.height,
      '--fx-carry': String(dir * (0.4 + rand() * 0.7 + (1 - s.dist) * 0.4)),
      '--fx-sx': String(dir * rand() * 0.4 + ux * (0.5 + rand() * 0.6)),
      '--fx-sy': String(0.4 + rand() * 0.8 + Math.max(0, uy) * 0.6),
      '--fx-kick': String(Math.max(0, -uy) * 0.9 + rand() * 0.5),
      '--fx-sr': String((rand() * 2 - 1) * (1.6 - s.dist)),
      '--fx-d': String(s.dist),
    }, fall, into);
  });
  const flash = piece('fx-impact', { left: strike.x, top: strike.y, '--fx-size': size }, '', into);
  const dust = [0, 1, 2, 3].map((i) => piece('fx-dust', {
    left: strike.x, top: strike.y, '--fx-size': size * (0.6 + rand() * 0.4), '--fx-i': String(i),
    '--fx-gx': String(dir * (0.3 + rand() * 0.8)), '--fx-gy': String(rand() * 1.2 - 0.8),
  }, '', into));
  return [...bits, flash, ...dust];
}
