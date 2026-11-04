// The server's event stream. EventSource sends Last-Event-ID itself after a reconnect, so missed events arrive late, not never.
import { BASE } from './client';

export type EventType =
  | 'job.created' | 'job.progress' | 'job.state' | 'job.conflict' | 'job.done' | 'fs.changed' | 'trash.changed' | 'labels.changed'
  | 'undo.changed' | 'share.opened' | 'unraid.parity' | 'unraid.spin' | 'index.progress' | 'search.hits' | 'prefs.changed' | 'notify' | 'heartbeat';

type Handler = (data: unknown) => void;
const handlers = new Map<EventType, Set<Handler>>();
let source: EventSource | null = null;

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
      const data = e.data ? JSON.parse(e.data) : null;
      handlers.get(type)?.forEach((fn) => fn(data));
    });
  }
  return listeners.get(type)!;
}

export function connect() {
  if (source) return;
  source = new EventSource(BASE + '/events');
  for (const type of handlers.keys()) source.addEventListener(type, listener(type));
}

export function disconnect() {
  source?.close();
  source = null;
}
