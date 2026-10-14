// Service worker веб-приложения. Задача — чтобы установленное приложение
// открывалось быстро и не застревало на старой версии после деплоя:
// страница и файлы без хеша в имени всегда берутся из сети (кэш — только на
// случай, когда сети нет), а файлы сборки с хешем в имени не меняются и
// берутся из кэша.
const CACHE_NAME = 'pm-dashboard-v3.1.0'
const SHELL = ['/index.html', '/manifest.json', '/favicon.svg']

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.addAll(SHELL)))
  self.skipWaiting()
})

// Кэши прежних версий удаляются
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key)))
    )
  )
  self.clients.claim()
})

// Файлы, имя которых меняется при каждой сборке (vite кладёт их в /assets/
// с хешем), и шрифты — их содержимое под тем же адресом не меняется
function immutable(url) {
  return url.pathname.startsWith('/assets/') || url.pathname.startsWith('/fonts/')
}

function remember(request, response) {
  if (response.ok && response.type === 'basic') {
    const clone = response.clone()
    caches.open(CACHE_NAME).then((cache) => cache.put(request, clone))
  }
  return response
}

self.addEventListener('fetch', (event) => {
  const { request } = event
  if (request.method !== 'GET') return
  const url = new URL(request.url)
  // Чужие адреса, API и WebSocket — мимо кэша
  if (url.origin !== self.location.origin) return
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/ws')) return

  // Страница: из сети, без сети — сохранённая копия
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response.ok) {
            const clone = response.clone()
            caches.open(CACHE_NAME).then((cache) => cache.put('/index.html', clone))
          }
          return response
        })
        .catch(() => caches.match('/index.html'))
    )
    return
  }

  // Файлы сборки: из кэша, иначе из сети с сохранением
  if (immutable(url)) {
    event.respondWith(
      caches.match(request).then((cached) => cached || fetch(request).then((response) => remember(request, response)))
    )
    return
  }

  // Остальное (манифест, иконки): из сети, без сети — из кэша
  event.respondWith(
    fetch(request)
      .then((response) => remember(request, response))
      .catch(() => caches.match(request))
  )
})
