// The fixture's clock. On its own it is the real one. The parity runner sets it to the page's time and from then on
// moves it along with the page's stopped clock, so jobs, the parity check and anything made carry the time the page
// shows, and a wait of a few seconds in a step file is a few seconds for the fixture too.
const timers = new Map();
let at = null;
let seq = 0;

const handle = (id) => ({ id, unref() { return this; }, ref() { return this; } });

function add(fn, ms, every) {
  if (at === null) {
    const t = every ? setInterval(fn, every) : setTimeout(fn, ms);
    t.unref?.();
    return t;
  }
  const id = ++seq;
  timers.set(id, { due: at + Math.max(0, ms), fn, every });
  return handle(id);
}

export const clock = {
  now: () => at ?? Date.now(),
  setTimeout: (fn, ms = 0) => add(fn, ms, 0),
  setInterval: (fn, ms) => add(fn, ms, ms),
  clear(t) {
    if (t && typeof t.id === 'number' && timers.has(t.id)) timers.delete(t.id);
    else { clearTimeout(t); clearInterval(t); }
  },
  // Runs every timer that falls due up to the given moment, in order, each at its own time.
  moveTo(to) {
    if (at === null) { at = to; return; }
    for (;;) {
      let next = null;
      for (const [id, t] of timers) if (t.due <= to && (!next || t.due < next[1].due)) next = [id, t];
      if (!next) break;
      const [id, t] = next;
      at = t.due;
      if (t.every) t.due += t.every;
      else timers.delete(id);
      t.fn();
    }
    at = Math.max(at, to);
  },
};

// PUT /api/v1/fixture/clock with { now } from the runner.
export function clockMiddleware() {
  return (req, res, next) => {
    if (req.method !== 'PUT' || !req.url.startsWith('/api/v1/fixture/clock')) return next();
    let raw = '';
    req.on('data', (c) => { raw += c; });
    req.on('end', () => {
      clock.moveTo(Number(JSON.parse(raw || '{}').now) || Date.now());
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify({ now: clock.now() }));
    });
  };
}
