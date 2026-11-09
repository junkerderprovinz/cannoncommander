// The page a client sends somebody to when it signs in through OAuth. The server serves the app there, and the query
// it came with is the request to answer.
const KEY = 'cannoncommander-consent';

export const onConsentPage = () => /\/oauth\/authorize\/?$/.test(location.pathname);

// A sign-in through another provider comes back to the page without its query, so the query waits here for the trip.
export function keepConsentQuery() {
  if (!location.search) return;
  try { sessionStorage.setItem(KEY, location.search); } catch { /* such a sign-in then starts over in the client */ }
}

export function consentQuery(): string {
  if (location.search) return location.search;
  try { return sessionStorage.getItem(KEY) ?? ''; } catch { return ''; }
}

export function forgetConsentQuery() {
  try { sessionStorage.removeItem(KEY); } catch { /* nothing kept, nothing to forget */ }
}

// The answer leaves the app for the address the client registered.
export const returnTo = (url: string) => location.assign(url);
