// The wire to one shell on the server. Typing goes out as bytes and a new size as text; the code the server closes
// with says why the terminal ended.
import { BASE } from '@/api/client';
import { t } from '@/i18n';

export type Ended = { exited: true } | { exited: false; text: string };
export interface Link { send(text: string): void; resize(cols: number, rows: number): void; close(): void }
export interface LinkEvents { open(): void; data(bytes: Uint8Array): void; end(e: Ended): void }

// The server takes 64 KB in one message, so a long paste goes in pieces well below that.
const PIECE = 16384;
const encoder = new TextEncoder();

export function socketUrl(path: string, cols: number, rows: number) {
  const params = new URLSearchParams({ path, cols: String(cols), rows: String(rows) });
  return `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}${BASE}/terminal?${params}`;
}

function why(code: number, reason: string): Ended {
  if (code === 1000 && reason === 'exited') return { exited: true };
  if (code === 4001) return { exited: false, text: t('terminal.ended.idle') };
  if (code === 4003) return { exited: false, text: t(`terminal.ended.${['signedOut', 'off'].includes(reason) ? reason : 'denied'}`) };
  if (code === 1001) return { exited: false, text: t('terminal.ended.stopping') };
  return { exited: false, text: t('terminal.ended.lost') };
}

export function link(path: string, cols: number, rows: number, on: LinkEvents): Link {
  const ws = new WebSocket(socketUrl(path, cols, rows));
  ws.binaryType = 'arraybuffer';
  let leaving = false;
  ws.onopen = () => on.open();
  ws.onmessage = (e) => { if (typeof e.data !== 'string') on.data(new Uint8Array(e.data as ArrayBuffer)); };
  ws.onclose = (e) => { if (!leaving) on.end(why(e.code, e.reason)); };
  const open = () => ws.readyState === WebSocket.OPEN;
  return {
    send(text) {
      if (!open()) return;
      const bytes = encoder.encode(text);
      for (let at = 0; at < bytes.length; at += PIECE) ws.send(bytes.subarray(at, at + PIECE));
    },
    resize(c, r) { if (open()) ws.send(JSON.stringify({ cols: c, rows: r })); },
    close() {
      leaving = true;
      ws.close(1000);
    },
  };
}
