import { ref } from 'vue'
import i18n from '../i18n'

export function formatMoney(amount: number): string {
  return amount.toLocaleString('ru-RU', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }) + ' ₽'
}

// Часовой пояс, в котором показывается время: выбор пользователя или пояс от
// администратора. Пусто — пояс браузера. Задаётся хранилищем настроек.
const timeZone = ref('')

function isKnownTimeZone(name: string): boolean {
  try {
    new Intl.DateTimeFormat(undefined, { timeZone: name })
    return true
  } catch {
    return false
  }
}

// Имя, которого браузер не знает, не применяется: время осталось бы без показа
export function setTimeZone(name: string) {
  timeZone.value = name && isKnownTimeZone(name) ? name : ''
}

export function browserTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone
}

// Все пояса, которые знает браузер; в старых браузерах — короткий список
export function timeZoneList(): string[] {
  const all: string[] | undefined = (Intl as any).supportedValuesOf?.('timeZone')
  return all && all.length ? all : [
    'UTC', 'Europe/Kaliningrad', 'Europe/Moscow', 'Europe/Samara', 'Asia/Yekaterinburg', 'Asia/Omsk',
    'Asia/Novosibirsk', 'Asia/Krasnoyarsk', 'Asia/Irkutsk', 'Asia/Yakutsk', 'Asia/Vladivostok',
    'Asia/Magadan', 'Asia/Kamchatka', 'Europe/London', 'Europe/Berlin', 'America/New_York',
  ]
}

// Даты и время пишутся по правилам языка интерфейса
function localeTag(): string {
  return i18n.global.locale.value === 'en' ? 'en-GB' : 'ru-RU'
}

// year: год четырьмя цифрами, двумя или без года
type YearStyle = 'numeric' | '2-digit' | 'none'

// Момент времени — в поясе пользователя. empty — что показать вместо пустого
// или непонятного значения.
export function formatDateTime(
  value: string | Date | null | undefined,
  options: { year?: YearStyle; seconds?: boolean; empty?: string } = {},
): string {
  const empty = options.empty ?? '—'
  if (!value) return empty
  const d = typeof value === 'string' ? new Date(value) : value
  if (isNaN(d.getTime())) return empty
  const format: Intl.DateTimeFormatOptions = { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }
  if (options.year !== 'none') format.year = options.year || 'numeric'
  if (options.seconds) format.second = '2-digit'
  if (timeZone.value) format.timeZone = timeZone.value
  return d.toLocaleString(localeTag(), format)
}

// Календарная дата без времени суток (срок задачи, дата счёта): «2026-10-13»
// или та же дата с нулевым временем, как её отдаёт сервер
const CALENDAR_DATE = /^(\d{4})-(\d{2})-(\d{2})(?:T00:00:00(?:\.0+)?(?:Z|[+-]00:00)?)?$/

// Календарная дата показывается как есть, без пересчёта пояса — иначе к
// западу от Гринвича она сдвинулась бы на день назад. Момент времени — в
// поясе пользователя.
export function formatDate(date: string | Date | null | undefined, year: YearStyle = 'numeric'): string {
  if (!date) return '—'
  const format: Intl.DateTimeFormatOptions = { day: '2-digit', month: '2-digit' }
  if (year !== 'none') format.year = year
  let d: Date
  const calendar = typeof date === 'string' ? CALENDAR_DATE.exec(date) : null
  if (calendar) {
    d = new Date(Date.UTC(Number(calendar[1]), Number(calendar[2]) - 1, Number(calendar[3])))
    format.timeZone = 'UTC'
  } else {
    d = typeof date === 'string' ? new Date(date) : date
    if (timeZone.value) format.timeZone = timeZone.value
  }
  if (isNaN(d.getTime())) return '—'
  return d.toLocaleDateString(localeTag(), format)
}

export function formatNumber(n: number): string {
  return n.toLocaleString('ru-RU', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  })
}

export function formatHours(hours: number | null): string {
  if (hours === null || hours === undefined) return '—'
  const h = Math.floor(hours)
  const m = Math.round((hours - h) * 60)
  return `${h}:${m.toString().padStart(2, '0')} ч.`
}
