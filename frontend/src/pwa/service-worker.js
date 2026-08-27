const CACHE_PREFIX = 'novelreader-shell-';
const CACHE_NAME = `${CACHE_PREFIX}__NOVELREADER_BUILD_ID__`;
const SHELL_ASSETS = __NOVELREADER_SHELL_ASSETS__;

function isApiPath(pathname) {
  return pathname === '/api' || pathname.startsWith('/api/');
}

async function installShell() {
  const cache = await caches.open(CACHE_NAME);
  await cache.addAll(SHELL_ASSETS);
}

async function activateShell() {
  const cacheNames = await caches.keys();
  await Promise.all(
    cacheNames
      .filter((name) => name.startsWith(CACHE_PREFIX) && name !== CACHE_NAME)
      .map((name) => caches.delete(name)),
  );
  await self.clients.claim();
}

async function networkFirstNavigation(request) {
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(CACHE_NAME);
      await cache.put('/index.html', response.clone());
    }
    return response;
  } catch {
    const cache = await caches.open(CACHE_NAME);
    return (await cache.match('/index.html')) || Response.error();
  }
}

async function cachedStaticAsset(request) {
  const cache = await caches.open(CACHE_NAME);
  const cached = await cache.match(request);
  if (cached) return cached;

  const response = await fetch(request);
  if (response.ok) await cache.put(request, response.clone());
  return response;
}

self.addEventListener('install', (event) => {
  event.waitUntil(installShell());
});

self.addEventListener('activate', (event) => {
  event.waitUntil(activateShell());
});

self.addEventListener('message', (event) => {
  if (event.data === 'activate-update') void self.skipWaiting();
});

self.addEventListener('fetch', (event) => {
  const { request } = event;
  if (request.method !== 'GET') return;

  const url = new URL(request.url);
  if (url.origin !== self.location.origin || isApiPath(url.pathname)) return;

  if (request.mode === 'navigate') {
    event.respondWith(networkFirstNavigation(request));
    return;
  }

  const isShellAsset = url.pathname.startsWith('/assets/')
    || url.pathname.startsWith('/icons/')
    || url.pathname === '/manifest.webmanifest';
  if (isShellAsset) event.respondWith(cachedStaticAsset(request));
});
