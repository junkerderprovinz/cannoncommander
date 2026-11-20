import { go } from '@/core/views';

const BIN = /(^|\/)\.cc-trash(\/|$)/;

// intoBin turns a typed path into a share's trash folder into the Papierkorb page, the one way in, since the server
// keeps the folder itself closed.
export function intoBin(path: string): false | undefined {
  if (!BIN.test(path)) return undefined;
  go('trash');
  return false;
}
