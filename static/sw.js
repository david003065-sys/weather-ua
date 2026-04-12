/**
 * @file Weather UA — service worker (scope must be site root; registered as `/sw.js`).
 * Precaches static assets, caches HTML navigations and weather API GET responses with size caps.
 */
const CACHE_NAME = "weather-ua-precache-v17";
const DYNAMIC_CACHE = "weather-ua-pages-v17";
const API_CACHE = "weather-ua-api-v17";
const OFFLINE_URL = "/static/offline.html";
const MAX_CACHE_ITEMS = 50;

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

/**
 * Whether a fetched response is safe to store in the page/dynamic cache.
 * @param {Response|null|undefined} response
 * @returns {boolean}
 */
function cacheableStaticResponse(response) {
  if (!response || !response.ok) return false;
  if (response.type !== "basic" && response.type !== "cors") return false;
  return true;
}

/**
 * Whether a weather API response is OK to persist in the API cache.
 * @param {Response|null|undefined} response
 * @returns {boolean}
 */
function cacheableAPIResponse(response) {
  if (!response || !response.ok) return false;
  if (response.type !== "basic" && response.type !== "cors") return false;
  return true;
}

/**
 * @param {string} pathname URL pathname (e.g. `/api/weather/kyiv`).
 * @returns {boolean} True if this request should use the API cache strategy.
 */
function isWeatherAPIRequest(pathname) {
  return pathname.startsWith("/api/weather/") || pathname.startsWith("/api/place_weather");
}

/**
 * Enforces an upper bound on the number of entries in a named Cache Storage bucket (eviction).
 *
 * **Algorithm (FIFO-style eviction via Cache API ordering)**:
 * 1. Open the cache by `cacheName` and call `cache.keys()`, which returns `Request` objects in
 *    **insertion order** (oldest entries first, per Cache Storage specification).
 * 2. If `keys.length <= maxItems`, do nothing — the bucket is within budget.
 * 3. Otherwise compute `toDelete = keys.length - maxItems` (exactly how many entries exceed the cap).
 * 4. Delete the first `toDelete` keys in that ordered list: `keys[0] … keys[toDelete-1]`.
 *    Those correspond to the **oldest** cached responses (first inserted), i.e. a **FIFO** eviction
 *    policy: new writes happen after `put()`, then `trimCache` removes surplus from the **front**
 *    of the queue. This is a simple bounded cache — not LRU by URL recency of *use*, only by
 *    insertion order into this specific Cache object.
 * 5. Each deletion is awaited sequentially to avoid overwhelming the storage backend; order is preserved.
 *
 * Called after a successful `cache.put` for navigations (`DYNAMIC_CACHE`) and weather API
 * (`API_CACHE`) so the cache does not grow without bound (`MAX_CACHE_ITEMS`).
 *
 * @param {string} cacheName CacheStorage name (e.g. `DYNAMIC_CACHE`, `API_CACHE`).
 * @param {number} maxItems Maximum number of entries to retain after trimming.
 * @returns {Promise<void>}
 */
async function trimCache(cacheName, maxItems) {
  const cache = await caches.open(cacheName);
  const keys = await cache.keys();
  if (keys.length <= maxItems) return;
  const toDelete = keys.length - maxItems;
  for (let i = 0; i < toDelete; i++) {
    await cache.delete(keys[i]);
  }
}

/**
 * Notifies all window clients that a fresh weather API response was written to the API cache.
 * @param {string} requestUrl Absolute URL that was cached.
 * @returns {Promise<void>}
 */
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
            await trimCache(DYNAMIC_CACHE, MAX_CACHE_ITEMS);
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

  // Weather API: network-first strategy (always fetch fresh data, fallback to cache)
  if (isWeatherAPIRequest(url.pathname)) {
    event.respondWith(
      (async () => {
        const apiCache = await caches.open(API_CACHE);

        // Try network first
        try {
          const networkResponse = await fetch(request);
          if (cacheableAPIResponse(networkResponse)) {
            // Update cache with fresh data
            await apiCache.put(request, networkResponse.clone());
            await trimCache(API_CACHE, MAX_CACHE_ITEMS);
            event.waitUntil(notifyClientsAPIUpdated(request.url));
          }
          return networkResponse;
        } catch (networkError) {
          // Network failed: fallback to cache
          const cached = await apiCache.match(request);
          if (cached) {
            return cached;
          }
          // No cache available: return offline error
          return new Response('{"error":"offline"}', {
            status: 503,
            statusText: "Offline",
            headers: { "Content-Type": "application/json; charset=utf-8" },
          });
        }
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
