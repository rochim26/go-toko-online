// Minimal app-shell service worker.
const CACHE = 'mdt-v1';
const PRECACHE = ['/', '/static/css/app.css', '/static/js/app.js', '/manifest.webmanifest'];

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then(c => c.addAll(PRECACHE).catch(()=>{})));
  self.skipWaiting();
});
self.addEventListener('activate', (e) => {
  e.waitUntil(caches.keys().then(keys => Promise.all(keys.filter(k => k!==CACHE).map(k => caches.delete(k)))));
  self.clients.claim();
});
self.addEventListener('fetch', (e) => {
  const req = e.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);
  if (url.origin !== location.origin) return;
  // Network-first for HTML, cache-first for static
  if (req.headers.get('accept')?.includes('text/html')) {
    e.respondWith(fetch(req).catch(() => caches.match(req).then(r => r || caches.match('/'))));
    return;
  }
  if (url.pathname.startsWith('/static/')) {
    e.respondWith(caches.match(req).then(r => r || fetch(req).then(resp => {
      const copy = resp.clone();
      caches.open(CACHE).then(c => c.put(req, copy));
      return resp;
    })));
  }
});
