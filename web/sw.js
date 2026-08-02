/* Offline cache. Only registers in a secure context (see app.js), so this file
   does nothing until the server is served over HTTPS — e.g. with mkcert. Audio
   and sprites are immutable once generated, hence cache-first; everything else
   goes to the network first so a content change lands immediately. */

const CACHE = 'pokelecture-v1';

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(['./', './app.js', './styles.css', './icons.js', './sfx.js'])));
  self.skipWaiting();
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys().then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))),
  );
  self.clients.claim();
});

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);
  if (url.pathname.includes('/api/')) return;

  // includes(), not startsWith(): under /pokelecture/ the prefix is not at index 0.
  const immutable = url.pathname.includes('/audio/') || url.pathname.includes('/sprites/');

  if (immutable) {
    event.respondWith(
      caches.match(event.request).then((hit) => hit || fetch(event.request).then((res) => {
        const copy = res.clone();
        caches.open(CACHE).then((c) => c.put(event.request, copy));
        return res;
      })),
    );
    return;
  }

  event.respondWith(
    fetch(event.request)
      .then((res) => {
        const copy = res.clone();
        caches.open(CACHE).then((c) => c.put(event.request, copy));
        return res;
      })
      .catch(() => caches.match(event.request)),
  );
});
