// Service worker веб-приложения. Задача — чтобы установленное приложение
// открывалось быстро и не застревало на старой версии после деплоя:
// страница и файлы без хеша в имени всегда берутся из сети (кэш — только на
// случай, когда сети нет), а файлы сборки с хешем в имени не меняются и
// берутся из кэша.
// Новое имя сбрасывает кэш, куда раньше могла попасть страница вместо JS-файла
const CACHE_NAME = 'pm-dashboard-v3.1.6'
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
  // Открытым страницам сообщается, что push дошёл и показан ли он: так
  // проверка в настройках видит, где теряется уведомление
  const report = (message) =>
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windows) => windows.forEach((w) => w.postMessage(message)))
  event.waitUntil(
    self.registration
      .showNotification(data.title || 'PM Dashboard', {
        body: data.body || '',
        tag: data.tag || undefined,
        // Иконка на тёмной подложке: само окно уведомления рисует система
        // в своей теме, цвет ему задать нельзя
        icon: '/push-icon-192x192.png',
        badge: '/push-icon-192x192.png',
        data: { url: data.url || '/notifications' },
      })
      .then(() => report({ type: 'push-shown', tag: data.tag || '' }))
      .catch((err) => report({ type: 'push-error', tag: data.tag || '', message: String(err && err.message || err) }))
  )
})

// Нажатие на push: уведомление в колокольчике отмечается прочитанным, затем
// открытое окно приложения само переходит по ссылке (WindowClient.navigate
// в Safari нет), а если окна нет — открывается новое сразу по ссылке
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = event.notification.data?.url || '/notifications'
  const target = new URL(url, self.location.origin).href
  // Метка push — «n<номер уведомления>» (у проверочного push номера нет)
  const match = /^n(\d+)$/.exec(event.notification.tag || '')
  const id = match ? Number(match[1]) : 0
  const markRead = id
    ? fetch('/api/notifications/' + id + '/read', { method: 'POST', credentials: 'include' }).catch(() => {})
    : Promise.resolve()
  event.waitUntil(
    markRead.then(() =>
      self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windows) => {
        const open = windows.find((w) => new URL(w.url).origin === self.location.origin)
        if (open) {
          open.postMessage({ type: 'open-url', url, id })
          return open.focus()
        }
        return self.clients.openWindow(target)
      })
    )
  )
})
