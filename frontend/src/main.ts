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
