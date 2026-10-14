type EventCallback = (data: any) => void

// Задержка переподключения растёт с каждой неудачей, но не больше минуты.
// Попытки не кончаются: вкладка может простоять открытой, пока сервер
// перезапускают, и после этого должна снова получать события.
const RECONNECT_BASE_MS = 5000
const RECONNECT_MAX_MS = 60000

const listeners = new Map<string, Set<EventCallback>>()
let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let attempts = 0

function connect() {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${protocol}//${location.host}/ws`

  let socket: WebSocket
  try {
    socket = new WebSocket(url)
  } catch {
    scheduleReconnect()
    return
  }
  ws = socket

  socket.onopen = () => {
    attempts = 0
  }

  socket.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      listeners.get(msg.event)?.forEach(cb => cb(msg.data))
    } catch {}
  }

  socket.onclose = () => {
    // Закрылось прежнее соединение, а новое уже открыто — переподключаться не нужно
    if (ws !== socket) return
    ws = null
    scheduleReconnect()
  }

  socket.onerror = () => {
    socket.close()
  }
}

function scheduleReconnect() {
  if (reconnectTimer) return
  const delay = Math.min(RECONNECT_BASE_MS * Math.pow(1.5, attempts), RECONNECT_MAX_MS)
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    attempts++
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

let initialized = false

// Одно соединение на вкладку, общее для всех, кто подписан на события.
export function useWebSocket() {
  if (!initialized) {
    initialized = true
    connect()
  }
  return { on, off }
}
