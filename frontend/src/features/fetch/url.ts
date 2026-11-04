import { t } from '@/i18n';

// Only web addresses: the server refuses anything else, and local ones as well.
export function parseUrl(text: string) {
  try {
    const u = new URL(text.trim());
    return /^https?:$/.test(u.protocol) ? u : null;
  } catch {
    return null;
  }
}

// The server names the job host/file, older jobs only have the file.
export function jobText(name: string) {
  const cut = name.indexOf('/');
  return cut < 0 ? t('fetch.job', { what: name }) : t('fetch.jobFrom', { what: name.slice(cut + 1), host: name.slice(0, cut) });
}
