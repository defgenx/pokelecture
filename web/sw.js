/* Offline cache. Only registers in a secure context (see app.js), so this file
   does nothing until the server is served over HTTPS — e.g. with mkcert. Audio
   and sprites are immutable once generated, hence cache-first; everything else
   goes to the network first so a content change lands immediately. */

// v2: v1 was cache-first over responses it never checked, so a tablet that
// loaded the game while the voice track was missing from the server pinned a
// 404 for every /audio/ URL and kept replaying it from cache. Renaming the
// cache is what evicts those.
// v3: badges, bonus games, Conseil des 4 — evict the pre-league app shell.
// v4: Îles arc, Tour de Combat, shiny, tablet metas — same reason.
// v5: 8-badge order, tower floors, arcade, sentence builder, landscape layouts.
// v6: world map, battle lives, gigamax, new regions and games.
// v7: neural voice track (same URLs, new audio) and the arcade roster.
const CACHE = 'pokelecture-v7';

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

// keep stores a copy only of a response worth replaying: a 404 must never be
// cached, and cache.put() rejects outright on the 206 a media element gets.
function keep(request, res) {
  if (res && res.ok && res.status === 200 && res.type !== 'opaque') {
    const copy = res.clone();
    caches.open(CACHE).then((c) => c.put(request, copy)).catch(() => {});
  }
  return res;
}

self.addEventListener('fetch', (event) => {
  const request = event.request;
  if (request.method !== 'GET') return;

  const url = new URL(request.url);
  if (url.pathname.includes('/api/')) return;

  // An <audio> element asks for byte ranges. Answering one from the cache hands
  // back a whole-file 200 to a request that asked for a slice, which Android
  // Chrome treats as a broken file — so leave range requests to the network,
  // where the 7-day Cache-Control header already covers repeat plays.
  if (request.headers.has('range')) return;

  // includes(), not startsWith(): under /pokelecture/ the prefix is not at index 0.
  const immutable = url.pathname.includes('/audio/') || url.pathname.includes('/sprites/');

  if (immutable) {
    event.respondWith(
      caches.match(request).then((hit) => hit || fetch(request).then((res) => keep(request, res))),
    );
    return;
  }

  event.respondWith(
    fetch(request)
      .then((res) => keep(request, res))
      .catch(() => caches.match(request)),
  );
});
