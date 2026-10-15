import { ref } from 'vue'
import axios from 'axios'

// Push-уведомления на это устройство: те же, что приходят в колокольчик.
// На iPhone и iPad push работает только в приложении, установленном на экран
// «Домой», и разрешение можно спросить только по нажатию кнопки.

export type PushState = 'unsupported' | 'install' | 'denied' | 'off' | 'on'

const state = ref<PushState>('off')
const busy = ref(false)

const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
const standalone = window.matchMedia('(display-mode: standalone)').matches || (navigator as any).standalone === true

// Ключ VAPID приходит в base64url, подписке он нужен байтами
function keyBytes(base64: string): ArrayBuffer {
  const padded = (base64 + '='.repeat((4 - (base64.length % 4)) % 4)).replace(/-/g, '+').replace(/_/g, '/')
  const binary = atob(padded)
  const bytes = new Uint8Array(new ArrayBuffer(binary.length))
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes.buffer
}

async function registration() {
  return navigator.serviceWorker.ready
}

async function refresh() {
  if (!('serviceWorker' in navigator) || !('PushManager' in window) || !('Notification' in window)) {
    state.value = isIOS && !standalone ? 'install' : 'unsupported'
    return
  }
  if (Notification.permission === 'denied') {
    state.value = 'denied'
    return
  }
  try {
    const sub = await (await registration()).pushManager.getSubscription()
    state.value = sub ? 'on' : 'off'
  } catch {
    state.value = 'off'
  }
}

// Включение — только по нажатию: браузер спрашивает разрешение
async function enable() {
  busy.value = true
  try {
    const permission = await Notification.requestPermission()
    if (permission !== 'granted') {
      state.value = permission === 'denied' ? 'denied' : 'off'
      return
    }
    const { data } = await axios.get('/api/push/key')
    const reg = await registration()
    const sub = (await reg.pushManager.getSubscription()) ||
      (await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: keyBytes(data.key) }))
    await axios.post('/api/push/subscribe', sub.toJSON())
    state.value = 'on'
  } finally {
    busy.value = false
  }
}

async function disable() {
  busy.value = true
  try {
    const sub = await (await registration()).pushManager.getSubscription()
    if (sub) {
      await axios.post('/api/push/unsubscribe', { endpoint: sub.endpoint }).catch(() => {})
      await sub.unsubscribe()
    }
    state.value = 'off'
  } finally {
    busy.value = false
  }
}

// Проверка: сервер отправляет push на все устройства пользователя и
// возвращает ответы сервисов доставки
export interface PushTestResult {
  service: string
  status: number
  error?: string
}

// Сервер создаёт настоящее уведомление по задаче пользователя и отправляет
// его push; нажатие на push откроет эту задачу
async function sendTest(): Promise<PushTestResult[]> {
  const { data } = await axios.post('/api/push/test')
  return data.results || []
}

export function usePush() {
  return { state, busy, refresh, enable, disable, sendTest }
}
