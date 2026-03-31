/* Weather UA — service worker (scope must be site root; registered as /sw.js) */
const CACHE_NAME = "weather-ua-precache-v17";
const DYNAMIC_CACHE = "weather-ua-pages-v17";
const API_CACHE = "weather-ua-api-v17";
const OFFLINE_URL = "/static/offline.html";

const PRECACHE_URLS = [
  "/manifest.webmanifest",
  "/static/style.css",
  "/static/atmosphere.css",
  "/static/atmosphere.js",
  "/static/script.js",
  "/static/theme.js",
  "/static/pwa.js",
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
      const allowList = new Set([CACHE_NAME, DYNAMIC_CACHE, API_CACHE]);
      const keys = await caches.keys();
      await Promise.all(
        keys.map((key) => (!allowList.has(key) ? caches.delete(key) : Promise.resolve()))
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

function cacheableAPIResponse(response) {
  if (!response || !response.ok) return false;
  if (response.type !== "basic" && response.type !== "cors") return false;
  return true;
}

function isWeatherAPIRequest(pathname) {
  return pathname.startsWith("/api/weather/") || pathname.startsWith("/api/place_weather");
}

async function notifyClientsAPIUpdated(requestUrl) {
  const clients = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
  const payload = {
    type: "weather-api-cache-updated",
    url: requestUrl,
    at: Date.now(),
  };
  for (const client of clients) {
    client.postMessage(payload);
  }
}

self.addEventListener("fetch", (event) => {
  const { request } = event;
  if (request.method !== "GET") return;

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;

  const accept = request.headers.get("accept") || "";
  const isDocument =
    request.mode === "navigate" ||
    request.destination === "document" ||
    (accept.includes("text/html") && !accept.includes("application/json"));

  if (isDocument) {
    event.respondWith(
      (async () => {
        const pageCache = await caches.open(DYNAMIC_CACHE);
        try {
          const network = await fetch(request);
          if (cacheableStaticResponse(network)) {
            await pageCache.put(request, network.clone());
          }
          return network;
        } catch (_) {
          const cached = await pageCache.match(request);
          if (cached) return cached;
          const precached = await caches.open(CACHE_NAME);
          const sameUrlCached = await precached.match(request);
          if (sameUrlCached) return sameUrlCached;
          const offline = await precached.match(OFFLINE_URL);
          if (offline) return offline;
          return new Response("", { status: 503, statusText: "Offline" });
        }
      })()
    );
    return;
  }

  if (isWeatherAPIRequest(url.pathname)) {
    event.respondWith(
      (async () => {
        const apiCache = await caches.open(API_CACHE);
        const cached = await apiCache.match(request);
        const networkPromise = fetch(request)
          .then(async (response) => {
            if (cacheableAPIResponse(response)) {
              await apiCache.put(request, response.clone());
              event.waitUntil(notifyClientsAPIUpdated(request.url));
            }
            return response;
          })
          .catch(() => null);

        if (cached) {
          event.waitUntil(networkPromise);
          return cached;
        }

        const network = await networkPromise;
        if (network) return network;
        return new Response('{"error":"offline"}', {
          status: 503,
          statusText: "Offline",
          headers: { "Content-Type": "application/json; charset=utf-8" },
        });
      })()
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
