// The desktop app's window has no frame on Windows and Linux, so the page draws the title bar and the window is moved
// and resized from the page; see openWindow in desktop/main.go. Moving and resizing go to the window over the web
// view's message channel, as Wails' own runtime would send them, and the buttons go through the server. The channel is
// used directly because Wails hands the page its postMessage unbound as `_wails.invoke` on Windows.
import { call, json } from '@/api/client';
import { host } from '@/core/host';

export const drawsOwnFrame = () => host.desktop && host.os !== 'darwin';

type Channel = { postMessage(message: string): void };
type WebView = { chrome?: { webview?: Channel }; webkit?: { messageHandlers?: { external?: Channel } } };

function post(message: string) {
  const w = window as unknown as WebView;
  (w.chrome?.webview ?? w.webkit?.messageHandlers?.external)?.postMessage(message);
}

export const frame = {
  maximised: async () => (await call<{ maximised: boolean }>('/window')).body?.maximised === true,
  setMaximised: (maximised: boolean) => call('/window', json('PUT', { maximised })),
  minimise: () => call('/window/minimise', { method: 'POST' }),
  // Asks the way the frame's close button does, so a window that waits in the tray hides instead.
  close: () => call('/window/close', { method: 'POST' }),
  // Hands a press that has started to move to the system, which moves the window from there.
  drag: () => post('wails:drag'),
};

type Edge = '' | 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw';

// How close to a side, in CSS pixels, a press grabs it, and how far a corner reaches along the sides.
const EDGE = 5;
const CORNER = 15;

// The side or corner of a width by height window that the point x, y grabs.
export function edgeAt(x: number, y: number, width: number, height: number): Edge {
  const onV = y < EDGE ? 'n' : y >= height - EDGE ? 's' : '';
  const onH = x < EDGE ? 'w' : x >= width - EDGE ? 'e' : '';
  if (!onV && !onH) return '';
  const nearV = y < CORNER ? 'n' : y >= height - CORNER ? 's' : '';
  const nearH = x < CORNER ? 'w' : x >= width - CORNER ? 'e' : '';
  return ((onV || nearV) + (onH || nearH)) as Edge;
}

const CURSOR: Record<Exclude<Edge, ''>, string> = {
  n: 'ns-resize', s: 'ns-resize', e: 'ew-resize', w: 'ew-resize',
  ne: 'nesw-resize', sw: 'nesw-resize', nw: 'nwse-resize', se: 'nwse-resize',
};

// A frameless window has no border for Windows or GTK to grab, so the page shows the resize cursor at an edge and
// hands a press there to Wails, which starts the system's own resize. It returns what takes the edges back.
export function resizeFromEdges(): () => void {
  let edge: Edge = '';
  const move = (e: PointerEvent) => {
    const next = edgeAt(e.clientX, e.clientY, innerWidth, innerHeight);
    if (next === edge) return;
    edge = next;
    document.body.style.cursor = edge ? CURSOR[edge] : '';
  };
  const press = (e: PointerEvent) => {
    if (!edge || e.button !== 0) return;
    e.preventDefault();
    e.stopPropagation();
    post(`wails:resize:${edge}-resize`);
  };
  addEventListener('pointermove', move);
  addEventListener('pointerdown', press, true);
  return () => {
    removeEventListener('pointermove', move);
    removeEventListener('pointerdown', press, true);
    document.body.style.cursor = '';
  };
}
