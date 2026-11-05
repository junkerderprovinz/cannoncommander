// Takes what another app shares to CannonCommander, keeps it for the page and opens the app, which asks where the
// files go. Every other request passes by untouched, so the app works the same as without it.
const BOX = 'cannoncommander-shared';
const at = (path) => new URL(path, registration.scope).href;

addEventListener('install', () => skipWaiting());
addEventListener('activate', (e) => e.waitUntil(clients.claim()));
addEventListener('fetch', (e) => {
  if (e.request.method === 'POST' && new URL(e.request.url).pathname === new URL(at('teilen')).pathname) e.respondWith(keep(e.request));
});

// One share at a time: a new one replaces what the page has not picked up yet.
async function keep(request) {
  const form = await request.formData();
  await caches.delete(BOX);
  const box = await caches.open(BOX);
  // A share without files still sends the field, as one empty file without a name.
  const files = form.getAll('dateien').filter((f) => typeof f !== 'string' && f.name);
  await Promise.all(files.map((f, i) => box.put(at(`geteilt/${i}`), new Response(f, {
    headers: { 'Content-Type': f.type || 'application/octet-stream', 'X-Name': encodeURIComponent(f.name), 'X-Modified': String(f.lastModified) },
  }))));
  const note = Object.fromEntries(['titel', 'text', 'adresse'].map((k) => [k, form.get(k)]).filter(([, v]) => typeof v === 'string' && v.trim()));
  if (Object.keys(note).length) await box.put(at('geteilt/notiz'), Response.json(note));
  return Response.redirect(at('?geteilt'), 303);
}
