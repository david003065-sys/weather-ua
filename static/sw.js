/* Weather UA — service worker (scope must be site root; registered as /sw.js) */
const CACHE_NAME = "weather-ua-pwa-v8";
const OFFLINE_URL = "/static/offline.html";

const PRECACHE_URLS = [
  "/manifest.webmanifest",
  "/static/style.css",
  "/static/atmosphere.css",
  "/static/script.js",
  "/static/theme.js",
  "/static/manifest.json",
  "/static/favicon.svg",
  OFFLINE_URL,
  "/static/icons/icon-144.png",
  "/static/icons/icon-192.png",
  "/static/icons/icon-512.png",
  "/static/screenshots/wide.png",
  "/static/screenshots/mobile.png",
];

self.addEventListener("install", (event) => {
  self.skipWaiting();
  event.waitUntil(
    (async () => {
      const cache = await caches.open(CACHE_NAME);
      await Promise.allSettled(
        PRECACHE_URLS.map((url) =>
          cache.add(url).catch((err) => {
            console.error("[SW] precache skip:", url, err);
          })
        )
      );
    })()
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const keys = await caches.keys();
      await Promise.all(
        keys.map((key) => (key !== CACHE_NAME ? caches.delete(key) : Promise.resolve()))
      );
      await self.clients.claim();
    })()
  );
});

function cacheableStaticResponse(response) {
  if (!response || !response.ok) return false;
  if (response.type !== "basic" && response.type !== "cors") return false;
  return true;
}

self.addEventListener("fetch", (event) => {
  const { request } = event;
  if (request.method !== "GET") return;

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;

  if (url.pathname.startsWith("/api/")) return;

  const accept = request.headers.get("accept") || "";
  const isDocument =
    request.mode === "navigate" ||
    request.destination === "document" ||
    (accept.includes("text/html") && !accept.includes("application/json"));

  if (isDocument) {
    event.respondWith(
      fetch(request).catch(async () => {
        const cached = await caches.match(request);
        if (cached) return cached;
        const offline = await caches.match(OFFLINE_URL);
        if (offline) return offline;
        return new Response("", { status: 503, statusText: "Offline" });
      })
    );
    return;
  }

  if (!url.pathname.startsWith("/static/")) return;

  event.respondWith(
    caches.match(request).then((cached) => {
      if (cached) return cached;
      return fetch(request).then((response) => {
        if (cacheableStaticResponse(response)) {
          const copy = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
        }
        return response;
      });
    })
  );
});
