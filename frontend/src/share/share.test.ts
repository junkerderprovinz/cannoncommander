import { createApp, nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { addMessages } from '@/i18n';
import de from './de.json';
import en from './en.json';
import { canPreview } from './preview';
import SharePage from './SharePage.vue';

addMessages('share', { de, en });

type Answer = { status?: number; body?: unknown };
let script: Record<string, Answer | Answer[]> = {};
const sent: { key: string; token: string | null }[] = [];

beforeEach(() => {
  script = {};
  sent.length = 0;
  history.replaceState(null, '', '/s/k3x9qa-urlaub');
  vi.stubGlobal('fetch', async (url: string, init: RequestInit = {}) => {
    const key = `${init.method ?? 'GET'} ${url.replace('/api/v1/s/k3x9qa-urlaub', '')}`;
    sent.push({ key, token: new Headers(init.headers).get('X-Share-Token') });
    const entry = script[key];
    const a = (Array.isArray(entry) ? entry.shift() : entry) ?? { status: 404, body: { title: 'Not Found', status: 404 } };
    const status = a.status ?? 200;
    return new Response(JSON.stringify(a.body ?? {}), { status, headers: { 'Content-Type': status >= 400 ? 'application/problem+json' : 'application/json' } });
  });
});
afterEach(() => {
  vi.unstubAllGlobals();
  document.body.innerHTML = '';
});

const settle = async () => {
  for (let i = 0; i < 6; i++) { await new Promise((r) => setTimeout(r, 0)); await nextTick(); }
};
async function mount() {
  const el = document.createElement('div');
  document.body.append(el);
  createApp(SharePage).mount(el);
  await settle();
}
const text = (sel: string) => document.querySelector(sel)?.textContent?.trim();

describe('the page for visitors', () => {
  it('explains each way a link leads nowhere', async () => {
    for (const [type, title] of [['share-expired', 'Abgelaufen'], ['share-used-up', 'Aufgebraucht'], ['share-withdrawn', 'Nicht mehr da']]) {
      script['GET '] = { status: 410, body: { type, title: 'Gone', status: 410 } };
      await mount();
      expect(text('.sp-gone h1')).toBe(title);
      document.body.innerHTML = '';
    }
    script['GET '] = { status: 404, body: { title: 'Not Found', status: 404 } };
    await mount();
    expect(text('.sp-gone h1')).toBe('Nichts gefunden');
  });

  it('opens a link with its password and sends the token along', async () => {
    script['GET '] = [{ body: { slug: 'k3x9qa-urlaub', locked: true } }, { body: { slug: 'k3x9qa-urlaub', locked: false, kind: 'folder', name: 'Urlaub', message: 'Viel Spaß!', uploads: true, uploadRoom: 1024 } }];
    script['POST /unlock'] = [{ status: 403, body: { title: 'Forbidden', status: 403 } }, { body: { token: 'tok', expires: '2026-11-05T00:00:00Z' } }];
    script['GET /list?path=%2F'] = { body: { path: '/', name: 'Urlaub', items: [{ name: 'Abends', dir: true, size: 0, mtime: '2026-11-01T10:00:00Z' }, { name: 'Strand.jpg', dir: false, size: 2048, mtime: '2026-11-01T10:00:00Z' }] } };
    await mount();
    const pass = document.querySelector('#sp-pass') as HTMLInputElement;
    const submit = () => document.querySelector('.sp-lock')!.dispatchEvent(new Event('submit', { cancelable: true }));
    pass.value = 'regen';
    pass.dispatchEvent(new Event('input'));
    submit();
    await settle();
    expect(text('.sp-err')).toContain('stimmt nicht');
    pass.value = 'sonnenschein';
    pass.dispatchEvent(new Event('input'));
    submit();
    await settle();
    expect(text('.sp-name h1')).toBe('Urlaub');
    expect(text('.sp-msg p')).toBe('Viel Spaß!');
    expect([...document.querySelectorAll('.sp-open .sp-nm')].map((e) => e.textContent)).toEqual(['Abends', 'Strand.jpg']);
    expect(document.querySelector('.sp-up')).not.toBeNull();
    expect(sent.filter((s) => s.key !== 'POST /unlock').slice(1).every((s) => s.token === 'tok')).toBe(true);
    expect((document.querySelector('[data-sp="zip"]') as HTMLAnchorElement).getAttribute('href')).toBe('/api/v1/s/k3x9qa-urlaub/raw?path=%2F&download=1&format=zip');
  });

  it('offers a preview only for what the browser shows', () => {
    expect(['Strand.jpg', 'film.mp4', 'lied.flac', 'rechnung.pdf', 'notiz.md'].every(canPreview)).toBe(true);
    expect(['archiv.zip', 'brief.docx', 'abbild.iso'].some(canPreview)).toBe(false);
  });
});
