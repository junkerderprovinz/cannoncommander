// A WebSocket that records what the page sends and lets the test play the server.
export class FakeSocket {
  static all: FakeSocket[] = [];
  static readonly OPEN = 1;
  readyState = 0;
  binaryType = 'blob';
  sent: (string | Uint8Array)[] = [];
  closedWith: number | null = null;
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: unknown }) => void) | null = null;
  onclose: ((e: { code: number; reason: string }) => void) | null = null;
  constructor(readonly url: string) { FakeSocket.all.push(this); }
  send(data: string | Uint8Array) { this.sent.push(data); }
  close(code?: number) { this.closedWith = code ?? 1005; this.readyState = 3; }
  open() { this.readyState = 1; this.onopen?.(); }
  serverCloses(code: number, reason = '') { this.readyState = 3; this.onclose?.({ code, reason }); }
}
