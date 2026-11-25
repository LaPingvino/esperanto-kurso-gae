// The site's service worker, served at /sw.js (pwa.go fills in VERSION, one
// per deploy). It never serves old code after a deploy:
//   - pages (navigations) and the game's course API: network first; the copy
//     kept is only for when the network is down (the last-visited pages);
//   - /static/ files with ?v= (the deploy's hash) and the game's files with
//     ?v= (each file's content hash): cache first, they never change;
//   - everything else (POSTs: answers, votes, the game's sync; sign-in;
//     admin; htmx fragments): straight to the network, never kept.
// A new deploy has a new VERSION, so a new worker: it takes over at once
// (skipWaiting, clients.claim) and drops the old deploy's caches.
const VERSION = "__VERSION__";
const STATIC = "ek-statiko-" + VERSION; // this deploy's ?v= files
const PAGES = "ek-pagxoj";   // last-visited pages, network first
const API = "ek-api";        // the game's course API, network first
const PICS = "ek-bildoj";    // unversioned /static/ files (pictures)
const LUDO = "ek-ludo";      // the game's hashed files: one copy per file
const EXT = "ek-ekstere";    // pinned libraries and fonts from CDNs
const KEEP = [STATIC, PAGES, API, PICS, LUDO, EXT];
const LIMIT = { [PAGES]: 60, [API]: 120, [PICS]: 400 };
const PRECACHE = __PRECACHE__;

self.addEventListener("install", (e) => {
  self.skipWaiting();
  e.waitUntil(caches.open(STATIC).then((c) => c.addAll(PRECACHE)).catch(() => {}));
});

self.addEventListener("activate", (e) => {
  e.waitUntil((async () => {
    for (const k of await caches.keys()) {
      if (k.startsWith("ek-") && !KEEP.includes(k)) await caches.delete(k);
    }
    await self.clients.claim();
  })());
});

const ok = (r) => r && r.ok && r.status === 200 && !r.redirected && (r.type === "basic" || r.type === "cors");

async function trim(name) {
  const max = LIMIT[name];
  if (!max) return;
  const c = await caches.open(name);
  const keys = await c.keys();
  for (let i = 0; i < keys.length - max; i++) await c.delete(keys[i]);
}

async function put(name, req, res) {
  const c = await caches.open(name);
  await c.delete(req, { ignoreVary: true });
  await c.put(req, res);
  await trim(name);
}

// Network first: the network's answer when there is one (and a copy kept);
// the kept copy only when the network fails.
// Pages are asked with cache: "no-cache": the browser's HTTP cache may not
// answer for them without asking the server (App Engine's static files, the
// game's page among them, say max-age=600).
async function networkFirst(e, name, fallback) {
  try {
    const res = await fetch(e.request, e.request.mode === "navigate" ? { cache: "no-cache" } : undefined);
    if (ok(res)) e.waitUntil(put(name, e.request, res.clone()));
    return res;
  } catch (err) {
    const hit = await caches.match(e.request, { cacheName: name, ignoreVary: true });
    if (hit) return hit;
    if (fallback) {
      const f = await fallback();
      if (f) return f;
    }
    throw err;
  }
}

async function cacheFirst(e, name) {
  const hit = await caches.match(e.request, { cacheName: name });
  if (hit) return hit;
  const res = await fetch(e.request);
  if (ok(res)) e.waitUntil(put(name, e.request, res.clone()));
  return res;
}

// The game's files carry their content hash (?v=): kept until a build
// changes it, then the new one replaces the old (one copy per file, so the
// 25 MB wasm is downloaded once per build and never piles up).
async function ludoFile(e) {
  const hit = await caches.match(e.request, { cacheName: LUDO });
  if (hit) return hit;
  const res = await fetch(e.request);
  if (ok(res)) {
    const copy = res.clone(); // now, before the page reads res
    e.waitUntil((async () => {
      const c = await caches.open(LUDO);
      const path = new URL(e.request.url).pathname;
      for (const k of await c.keys()) if (new URL(k.url).pathname === path) await c.delete(k);
      await c.put(e.request, copy);
    })());
  }
  return res;
}

const offlinePage = () => new Response(
  '<!doctype html><html lang="eo"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">' +
  '<title>Eksterrete</title><body style="font:17px system-ui,sans-serif;background:#E6F2EE;color:#102C28;margin:0;padding:2rem">' +
  '<h1 style="color:#0F5024">Eksterrete</h1><p>Ĉi tiu paĝo ankoraŭ ne estas konservita en via aparato. ' +
  'La paĝoj, kiujn vi jam vizitis, kaj <a href="/ludo/">la ludo</a> funkcias ankaŭ sen reto.</p>' +
  '<p><a href="/">Al la ĉefpaĝo</a></p></body></html>',
  { status: 503, headers: { "Content-Type": "text/html; charset=utf-8" } });

const PINNED = /^https:\/\/(cdn\.jsdelivr\.net\/npm|unpkg\.com)\/[^/]+@\d/;

self.addEventListener("fetch", (e) => {
  const req = e.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);

  if (url.origin !== self.location.origin) {
    if (PINNED.test(req.url) || url.hostname === "fonts.gstatic.com") e.respondWith(cacheFirst(e, EXT));
    else if (url.hostname === "fonts.googleapis.com") e.respondWith(networkFirst(e, EXT));
    return;
  }
  const p = url.pathname;
  if (p === "/sw.js" || p.startsWith("/auth/") || p.startsWith("/admin") || p === "/enskribi") return;

  if (req.mode === "navigate") {
    e.respondWith(networkFirst(e, PAGES, async () => {
      // /ludo and /ludo/ are one page.
      if (p === "/ludo") return caches.match("/ludo/", { cacheName: PAGES, ignoreVary: true });
      return offlinePage();
    }));
    return;
  }
  if (p.startsWith("/ludo-api/")) { e.respondWith(networkFirst(e, API)); return; }
  if (p.startsWith("/ludo/") && url.searchParams.has("v")) { e.respondWith(ludoFile(e)); return; }
  if (p.startsWith("/static/")) {
    if (url.searchParams.has("v")) {
      // An offline page from before a deploy asks for the old ?v=: this
      // deploy's copy of the same file is the next best thing.
      e.respondWith(cacheFirst(e, STATIC).catch(() =>
        caches.match(req, { cacheName: STATIC, ignoreSearch: true }).then((r) => r || Response.error())));
    } else {
      e.respondWith(networkFirst(e, PICS));
    }
  }
});
