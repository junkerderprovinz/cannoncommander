// The terminal's WebSocket in the fixture, plain JavaScript for Vite's server like the rest of it.
import type { IncomingMessage } from 'node:http';
import type { Duplex } from 'node:stream';

export function terminalUpgrade(on: () => boolean): (req: IncomingMessage, socket: Duplex) => void;
