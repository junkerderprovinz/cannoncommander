// An account that signs in elsewhere proves it is at the keyboard with a fresh sign-in. The step it was taking waits
// here across the trip to the provider, and the card takes it up again on the way back.
import { signInAgain } from '../proof';

export type Step = { do: 'create'; client: string } | { do: 'write' | 'rotate'; id: string };

const KEY = 'cannoncommander-mcp-again';

export function signInFor(step: Step) {
  try { sessionStorage.setItem(KEY, JSON.stringify(step)); } catch { /* the card then only opens again */ }
  void signInAgain('mcp');
}

export function takeStep(): Step | null {
  try {
    const raw = sessionStorage.getItem(KEY);
    sessionStorage.removeItem(KEY);
    return raw ? (JSON.parse(raw) as Step) : null;
  } catch {
    return null;
  }
}
