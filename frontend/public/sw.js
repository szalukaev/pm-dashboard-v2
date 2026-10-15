// Service worker веб-приложения. Задача — чтобы установленное приложение
// открывалось быстро и не застревало на старой версии после деплоя:
// страница и файлы без хеша в имени всегда берутся из сети (кэш — только на
// случай, когда сети нет), а файлы сборки с хешем в имени не меняются и
// берутся из кэша.
// Новое имя сбрасывает кэш, куда раньше могла попасть страница вместо JS-файла
const CACHE_NAME = 'pm-dashboard-v3.1.3'
// Сколько ждать сеть, прежде чем показать сохранённую страницу: на плохой
// мобильной сети запрос иначе висит до тайм-аута телефона (минута и больше)
const NAVIGATE_TIMEOUT_MS = 4000
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
  // HTML под адресом файла сборки — это страница-заглушка, а не сам файл;
  // такое не сохраняем
  const html = (response.headers.get('Content-Type') || '').includes('text/html')
  if (response.ok && response.type === 'basic' && !(html && immutable(new URL(request.url)))) {
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

  // Страница: из сети; если сети нет или она не ответила вовремя — сохранённая
  // копия (а когда сеть всё же ответит, копия обновится)
  if (request.mode === 'navigate') {
    const network = fetch(request).then((response) => {
      if (response.ok) {
        const clone = response.clone()
        caches.open(CACHE_NAME).then((cache) => cache.put('/index.html', clone))
      }
      return response
    })
    const cached = () => caches.match('/index.html')
    const late = new Promise((resolve) => setTimeout(resolve, NAVIGATE_TIMEOUT_MS)).then(cached)
    event.respondWith(
      Promise.race([network, late])
        .then((response) => response || network)
        .catch(() => cached().then((copy) => copy || network))
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

// Push: то же уведомление, что пришло в колокольчик. tag не даёт показать
// одно уведомление дважды
self.addEventListener('push', (event) => {
  let data = {}
  try {
    data = event.data ? event.data.json() : {}
  } catch {
    data = { title: event.data ? event.data.text() : '' }
  }
  event.waitUntil(
    self.registration.showNotification(data.title || 'PM Dashboard', {
      body: data.body || '',
      tag: data.tag || undefined,
      icon: '/android-chrome-192x192.png',
      badge: '/android-chrome-192x192.png',
      data: { url: data.url || '/notifications' },
    })
  )
})

// Нажатие на push: открытое окно приложения переходит по ссылке, иначе
// открывается новое
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const target = new URL(event.notification.data?.url || '/notifications', self.location.origin).href
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windows) => {
      const open = windows.find((w) => new URL(w.url).origin === self.location.origin)
      if (open) {
        return open.focus().then((w) => (w || open).navigate(target))
      }
      return self.clients.openWindow(target)
    })
  )
})
