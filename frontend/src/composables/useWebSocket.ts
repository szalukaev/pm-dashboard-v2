import { ref, onUnmounted } from 'vue'

type EventCallback = (data: any) => void

const connected = ref(false)
const reconnecting = ref(false)
const reconnectAttempts = ref(0)
const MAX_RECONNECT_ATTEMPTS = 50
const listeners = new Map<string, Set<EventCallback>>()
let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

function connect() {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${protocol}//${location.host}/ws`

  try {
    ws = new WebSocket(url)
  } catch {
    scheduleReconnect()
    return
  }

  ws.onopen = () => {
    connected.value = true
    reconnecting.value = false
    reconnectAttempts.value = 0
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      const cbs = listeners.get(msg.event)
      if (cbs) {
        cbs.forEach(cb => cb(msg.data))
      }
    } catch {}
  }

  ws.onclose = () => {
    connected.value = false
    scheduleReconnect()
  }

  ws.onerror = () => {
    ws?.close()
  }
}

function scheduleReconnect() {
  if (reconnectAttempts.value >= MAX_RECONNECT_ATTEMPTS) {
    reconnecting.value = false
    return
  }
  reconnecting.value = true
  const delay = Math.min(5000 * Math.pow(1.5, reconnectAttempts.value), 60000)
  reconnectTimer = setTimeout(() => {
    reconnectAttempts.value++
    connect()
  }, delay)
}

function on(event: string, callback: EventCallback) {
  if (!listeners.has(event)) {
    listeners.set(event, new Set())
  }
  listeners.get(event)!.add(callback)
}

function off(event: string, callback: EventCallback) {
  listeners.get(event)?.delete(callback)
}

function manualReconnect() {
  reconnectAttempts.value = 0
  if (reconnectTimer) clearTimeout(reconnectTimer)
  connect()
}

let initialized = false

export function useWebSocket() {
  if (!initialized) {
    initialized = true
    connect()
  }

  onUnmounted(() => {
    // Don't disconnect — shared connection
  })

  return { connected, reconnecting, reconnectAttempts, on, off, manualReconnect }
}
