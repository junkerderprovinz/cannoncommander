// The server's event stream. EventSource sends Last-Event-ID itself after a reconnect, so missed events arrive late, not never.
// Each tab has its own client id; the folders it shows are registered under it, and fs.changed comes only for those.
import { api, BASE } from './client';

export type EventType =
  | 'job.created' | 'job.progress' | 'job.state' | 'job.conflict' | 'job.done' | 'fs.changed' | 'trash.changed' | 'labels.changed'
  | 'undo.changed' | 'share.opened' | 'unraid.parity' | 'unraid.spin' | 'index.progress' | 'search.hits' | 'prefs.changed' | 'notify'
  | 'heartbeat' | 'resync';

// One frame as the hub writes it: the topic names what it is about, such as fs:/mnt/user/filme.
export interface ServerEvent<T = unknown> { id?: number; type: EventType; topic?: string; time?: string; data?: T }

type Handler = (event: ServerEvent) => void;
const handlers = new Map<EventType, Set<Handler>>();
let source: EventSource | null = null;

function newClientId() {
  const id = crypto.randomUUID?.() ?? Math.random().toString(36).slice(2);
  try { sessionStorage.setItem('cannoncommander-client', id); } catch { /* the id lives as long as the page then */ }
  return id;
}
// A reload keeps the tab's id, so the server still has the folders it watched.
export const clientId = (() => {
  try { return sessionStorage.getItem('cannoncommander-client') || newClientId(); } catch { return newClientId(); }
})();

export function on(type: EventType, fn: Handler) {
  if (!handlers.has(type)) handlers.set(type, new Set());
  handlers.get(type)!.add(fn);
  source?.addEventListener(type, listener(type));
  return () => handlers.get(type)?.delete(fn);
}

const listeners = new Map<EventType, (e: MessageEvent) => void>();
function listener(type: EventType) {
  if (!listeners.has(type)) {
    listeners.set(type, (e: MessageEvent) => {
      const event: ServerEvent = e.data ? JSON.parse(e.data) : { type };
      handlers.get(type)?.forEach((fn) => fn(event));
    });
  }
  return listeners.get(type)!;
}

export function emit(event: ServerEvent) {
  handlers.get(event.type)?.forEach((fn) => fn(event));
}

let watched: string[] = [];
let sending: ReturnType<typeof setTimeout> | undefined;
// The folders on screen change in bursts while someone browses, so only the last set of a burst goes out.
export function watch(paths: string[]) {
  const next = [...new Set(paths)].filter((p) => !p.includes('::')).sort().slice(0, 64);
  if (next.join('\n') === watched.join('\n')) return;
  watched = next;
  clearTimeout(sending);
  sending = setTimeout(() => { if (source) api.watch(clientId, watched).catch(() => {}); }, 200);
}

export function connect() {
  if (source) return;
  source = new EventSource(BASE + '/events?' + new URLSearchParams({ clientId }));
  for (const type of handlers.keys()) source.addEventListener(type, listener(type));
  // The server forgets a client's folders two minutes after its stream ends, so every new stream names them again.
  source.addEventListener('open', () => { if (watched.length) api.watch(clientId, watched).catch(() => {}); });
}

export function disconnect() {
  source?.close();
  source = null;
}
