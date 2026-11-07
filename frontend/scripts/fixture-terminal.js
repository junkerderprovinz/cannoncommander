// The WebSocket of GET /api/v1/terminal without a server: a pretend shell that echoes what is typed, answers echo, and
// closes as the server does on exit, so the browser checks can open a terminal. Frames are few and small, so a plain
// reading of RFC 6455 is enough here.
import { createHash } from 'node:crypto';

const GUID = '258EAFA5-E914-47DA-95CA-C5AB0DC85B11';

function frame(op, payload) {
  const n = payload.length;
  const head = n < 126 ? Buffer.from([0x80 | op, n]) : Buffer.from([0x80 | op, 126, n >> 8, n & 255]);
  return Buffer.concat([head, payload]);
}

function closing(code, reason) {
  const body = Buffer.alloc(2 + Buffer.byteLength(reason));
  body.writeUInt16BE(code);
  body.write(reason, 2);
  return frame(8, body);
}

export function terminalUpgrade(on) {
  return (req, socket) => {
    const url = new URL(req.url ?? '/', 'http://fixture');
    // Vite's own socket for reloading the page passes by on the same server.
    if (url.pathname !== '/api/v1/terminal') return;
    if (!on()) {
      socket.end('HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n');
      return;
    }
    const accept = createHash('sha1').update(req.headers['sec-websocket-key'] + GUID).digest('base64');
    socket.write(['HTTP/1.1 101 Switching Protocols', 'Upgrade: websocket', 'Connection: Upgrade', `Sec-WebSocket-Accept: ${accept}`, '', ''].join('\r\n'));
    const here = url.searchParams.get('path') || '/';
    const send = (text) => socket.write(frame(2, Buffer.from(text)));
    const prompt = () => send(`${here} $ `);
    send(`fixture shell in ${here}, ${url.searchParams.get('cols')}x${url.searchParams.get('rows')}\r\n`);
    prompt();
    let line = '', pending = Buffer.alloc(0);
    const typed = (text) => {
      for (const ch of text) {
        if (ch === '\r') {
          send('\r\n');
          if (line === 'exit') return socket.end(closing(1000, 'exited'));
          if (line.startsWith('echo ')) send(line.slice(5) + '\r\n');
          else if (line) send(`${line.split(' ')[0]}: not in the fixture\r\n`);
          line = '';
          prompt();
        } else if (ch === '\x7f') {
          if (line) send('\b \b');
          line = line.slice(0, -1);
        } else if (ch >= ' ') {
          line += ch;
          send(ch);
        }
      }
    };
    socket.on('data', (chunk) => {
      pending = Buffer.concat([pending, chunk]);
      while (pending.length >= 2) {
        const op = pending[0] & 15;
        let n = pending[1] & 127, at = 2;
        if (n === 126) {
          if (pending.length < 4) return;
          n = pending.readUInt16BE(2);
          at = 4;
        }
        const mask = pending.subarray(at, at + 4);
        at += 4;
        if (pending.length < at + n) return;
        const data = Buffer.from(pending.subarray(at, at + n));
        pending = pending.subarray(at + n);
        for (let i = 0; i < data.length; i++) data[i] ^= mask[i & 3];
        if (op === 8) return socket.end(frame(8, data.subarray(0, 2)));
        if (op === 9) socket.write(frame(10, data));
        if (op === 2) typed(data.toString());
      }
    });
    socket.on('error', () => {});
  };
}
