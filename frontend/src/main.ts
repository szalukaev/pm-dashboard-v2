import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { Quasar } from 'quasar'
import '@quasar/extras/material-icons/material-icons.css'
import 'quasar/src/css/index.sass'
import App from './App.vue'
import router from './router'
import i18n from './i18n'
import './styles/tokens.css'
import './styles/global.css'
import './styles/admin.css'
// Тема и режим (в том числе «как в системе») — до первого экрана
import './composables/useTheme'

const app = createApp(App)
app.use(createPinia())
app.use(router)

// Файл вкладки не загрузился — обычно страница из старой сборки после деплоя.
// Один раз перезагружаем страницу целиком; флаг не даёт уйти в цикл, если
// файл недоступен по другой причине
const RELOAD_FLAG = 'pm-dashboard-chunk-reload'
function reloadOnce() {
  try {
    if (sessionStorage.getItem(RELOAD_FLAG)) return
    sessionStorage.setItem(RELOAD_FLAG, '1')
  } catch {
    return
  }
  window.location.reload()
}
window.addEventListener('vite:preloadError', reloadOnce)
router.onError((err) => {
  if (/dynamically imported module|Importing a module script failed|error loading dynamically/i.test(String(err?.message))) reloadOnce()
})
router.afterEach(() => {
  try { sessionStorage.removeItem(RELOAD_FLAG) } catch {}
})
app.use(i18n)
app.use(Quasar, { config: {} })
app.mount('#app')

// Новая сборка на сервере — предложить обновиться
import('./composables/useAppVersion').then(m => m.startVersionWatch())

// Регистрация service worker для PWA
if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch(() => {})
  })
}

// Переход по ссылке нажатого push. Если адрес уже тот же (карточка этой
// задачи открывалась раньше), router.push ничего не сделает — сначала
// уходим с него, чтобы карточка открылась снова
async function openPushUrl(url: string) {
  if (router.currentRoute.value.fullPath === url) {
    await router.replace(router.currentRoute.value.path)
  }
  router.push(url)
}

// Ссылка нажатого push, которую service worker запомнил на случай, если его
// сообщение до страницы не дошло. Забирается один раз; старая не нужна
const PENDING_MAX_AGE = 2 * 60 * 1000

async function takePendingPush(): Promise<string> {
  if (!('caches' in window)) return ''
  try {
    const cache = await caches.open('pmd-pending-open')
    const response = await cache.match('/__pending-open')
    if (!response) return ''
    await cache.delete('/__pending-open')
    const { url, at } = await response.json()
    if (typeof url !== 'string' || !url.startsWith('/') || Date.now() - at > PENDING_MAX_AGE) return ''
    return url
  } catch {
    return ''
  }
}

// Страница снова видна или только загрузилась: открыть потерянный переход.
// Окно, открытое сразу по ссылке push, уже на нужном адресе — повторять не надо
async function openPendingPush() {
  const url = await takePendingPush()
  if (url && url !== window.location.pathname + window.location.search) openPushUrl(url)
}

if ('serviceWorker' in navigator) {
  openPendingPush()
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') openPendingPush()
  })
}

// Service worker сообщает, что push дошёл до устройства и показан ли он
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.addEventListener('message', async (event) => {
    const data = event.data || {}
    // Нажали на push, а приложение уже открыто: переход по ссылке уведомления
    // (задача откроется карточкой), колокольчик — с отметкой «прочитано»
    if (data.type === 'open-url' && typeof data.url === 'string' && data.url.startsWith('/')) {
      await takePendingPush()
      if (data.id) {
        const { useNotificationsStore } = await import('./stores/notifications')
        useNotificationsStore().fetch()
      }
      openPushUrl(data.url)
      return
    }
    if (data.type !== 'push-shown' && data.type !== 'push-error') return
    // Тост — только для проверочного push из настроек
    if (!data.test) return
    const { useToast } = await import('./composables/useToast')
    const { t } = i18n.global
    if (data.type === 'push-shown') useToast().show(t('settings.notifications.push_received'), 'success')
    else useToast().show(t('settings.notifications.push_not_shown', { reason: data.message }), 'error')
  })
}
