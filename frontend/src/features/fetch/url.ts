// Only web addresses: the server refuses anything else, and local ones as well.
export function parseUrl(text: string) {
  try {
    const u = new URL(text.trim());
    return /^https?:$/.test(u.protocol) ? u : null;
  } catch {
    return null;
  }
}
