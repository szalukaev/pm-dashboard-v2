import i18n from '../i18n'
import { useToast } from './useToast'

// Проверка новой версии приложения: на сервере может оказаться сборка новее
// той, что открыта. Тогда появляется тост «Приложение обновлено» с кнопкой
// «Обновить»; закрыть его можно крестиком или смахиванием.

const CHECK_EVERY_MS = 5 * 60_000

let notified = false

async function check() {
  if (notified) return
  try {
    const response = await fetch('/version.json', { cache: 'no-store' })
    if (!response.ok) return
    const { build } = await response.json()
    if (!build || build === __BUILD_ID__) return
  } catch {
    return
  }
  // Один тост на версию; закрытый крестиком до перезагрузки не возвращается
  notified = true
  useToast().showPersistent(i18n.global.t('common.app_updated'), {
    label: i18n.global.t('common.app_update'),
    run: reloadFresh,
  })
}

// Обновление: service worker перечитывается, кэши удаляются, страница грузится заново
async function reloadFresh() {
  try {
    const registration = await navigator.serviceWorker?.getRegistration()
    await registration?.update()
  } catch {}
  try {
    const keys = await caches.keys()
    await Promise.all(keys.map(key => caches.delete(key)))
  } catch {}
  window.location.reload()
}

export function startVersionWatch() {
  setInterval(check, CHECK_EVERY_MS)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') check()
  })
  window.addEventListener('online', check)
}
