import { useRouter } from 'vue-router'

// «Назад» смахиванием от левого края экрана на телефоне. Сначала его
// получает то, что открыто поверх страницы (например, карточка задачи —
// она вернёт к предыдущей связанной задаче или закроется); если никто не
// взял — шаг назад по страницам приложения.

type BackHandler = () => boolean

const handlers: BackHandler[] = []

// Регистрирует обработчик «назад»; последний зарегистрированный — первый.
// Возвращает функцию снятия
export function onSwipeBack(handler: BackHandler): () => void {
  handlers.push(handler)
  return () => {
    const i = handlers.lastIndexOf(handler)
    if (i >= 0) handlers.splice(i, 1)
  }
}

const EDGE = 28 // px от левого края, откуда начинается жест
const DISTANCE = 70 // сколько провести вправо
let started = false

export function installSwipeBack(router: ReturnType<typeof useRouter>) {
  let x0 = 0
  let y0 = 0
  let tracking = false
  if (started) return
  started = true

  window.addEventListener('touchstart', e => {
    const t = e.touches[0]
    tracking = e.touches.length === 1 && t.clientX <= EDGE
    x0 = t.clientX
    y0 = t.clientY
  }, { passive: true })

  window.addEventListener('touchend', e => {
    if (!tracking) return
    tracking = false
    const t = e.changedTouches[0]
    const dx = t.clientX - x0
    const dy = Math.abs(t.clientY - y0)
    if (dx < DISTANCE || dy > dx * 0.6) return
    for (let i = handlers.length - 1; i >= 0; i--) {
      if (handlers[i]()) return
    }
    if (window.history.state?.back) router.back()
  }, { passive: true })
}
