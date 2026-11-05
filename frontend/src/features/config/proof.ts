// Before a new token or passkey the server wants to know the owner is at the keyboard: the current password, or for
// an account that signs in elsewhere, a sign in from the last five minutes.
import { auth } from '@/api/auth';
import { ApiError } from '@/api/client';
import { c } from './shared';

export type Again = 'token' | 'passkey';

export const staleSignIn = (e: unknown) => e instanceof ApiError && e.status === 403 && e.problem?.type === 'fresh-sign-in';

// The note for a refused password, or null when the refusal had another reason. The window checks for an empty
// field before it sends, so a 400 is about something else.
export function proofRefused(e: unknown): string | null {
  if (!(e instanceof ApiError)) return null;
  if (e.status === 403) return c('security.wrongCurrent');
  if (e.status === 429) return c('security.tooMany');
  return null;
}

// The provider sends the person back to the security tab, which opens the same window again.
export async function signInAgain(again: Again) {
  const url = (await auth.options().catch(() => null))?.oidc?.url ?? 'api/auth/oidc/login';
  location.assign(url + '?redirect=' + encodeURIComponent(location.pathname + '?again=' + again));
}

let pending: Again | null = null;

export function takeAgainFromUrl(): boolean {
  const params = new URLSearchParams(location.search);
  const again = params.get('again');
  if (again !== 'token' && again !== 'passkey') return false;
  pending = again;
  params.delete('again');
  const rest = params.toString();
  history.replaceState(history.state, '', location.pathname + (rest ? '?' + rest : '') + location.hash);
  return true;
}

export function takePending(): Again | null {
  const again = pending;
  pending = null;
  return again;
}
