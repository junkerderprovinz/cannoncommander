// The prototype's own functions, cut out of its source and run in a sandbox, so a test can hold the port against them.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';

const root = join(__dirname, '..', '..', '..', 'prototype', 'src', 'js');
export const prototypeSource = (file: string) => readFileSync(join(root, file), 'utf8').replace(/\r\n/g, '\n');

// The text from the line that starts with `from` up to and including the first line that starts with `to` after it.
export function cut(source: string, from: string, to: string) {
  const a = source.indexOf(from);
  if (a < 0) throw new Error('prototype changed, missing: ' + from);
  const b = source.indexOf(to, a + from.length);
  if (b < 0) throw new Error('prototype changed, missing end of: ' + from);
  return source.slice(a, b + to.length);
}

export const runPrototype = <T>(code: string, context: Record<string, unknown>): T => vm.runInNewContext(code, context) as T;
