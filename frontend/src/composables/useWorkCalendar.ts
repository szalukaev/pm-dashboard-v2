import { ref } from 'vue'
import axios from 'axios'

// Производственный календарь России для расчёта рабочих дней спринта. Сервер
// отдаёт год строкой: i-й символ — (i+1)-й день, '1' — выходной или праздник,
// '0' — рабочий. Год загружается один раз за сеанс.

export type DurationUnit = 'workdays' | 'calendar'

const years = new Map<number, Promise<string>>()
// Хоть один год пришёл без настоящего календаря: праздники не учтены
const fallback = ref(false)

function loadYear(year: number): Promise<string> {
  let promise = years.get(year)
  if (!promise) {
    promise = axios.get('/api/calendar/days', { params: { year } }).then(({ data }) => {
      if (data.source !== 'isdayoff') fallback.value = true
      return data.days as string
    })
    promise.catch(() => years.delete(year))
    years.set(year, promise)
  }
  return promise
}

// Даты ГГГГ-ММ-ДД считаются в UTC: так переход на летнее время не сдвигает дни
function parse(date: string): Date {
  return new Date(date + 'T00:00:00Z')
}

function format(d: Date): string {
  return d.toISOString().slice(0, 10)
}

function dayOfYear(d: Date): number {
  return Math.floor((d.getTime() - Date.UTC(d.getUTCFullYear(), 0, 1)) / 86_400_000)
}

async function isWorkday(d: Date): Promise<boolean> {
  const days = await loadYear(d.getUTCFullYear())
  return days[dayOfYear(d)] === '0'
}

// Не больше трёх лет вперёд: защита от бесконечного счёта
const MAX_DAYS = 3 * 366

// Дата окончания по началу и продолжительности; день начала — первый день
async function endFromDuration(start: string, duration: number, unit: DurationUnit): Promise<string> {
  const d = parse(start)
  if (unit === 'calendar') {
    d.setUTCDate(d.getUTCDate() + duration - 1)
    return format(d)
  }
  let counted = 0
  for (let i = 0; i < MAX_DAYS; i++, d.setUTCDate(d.getUTCDate() + 1)) {
    if (await isWorkday(d)) counted++
    if (counted >= duration) return format(d)
  }
  return format(d)
}

// Продолжительность между датами включительно
async function durationFromDates(start: string, end: string, unit: DurationUnit): Promise<number> {
  const from = parse(start)
  const to = parse(end)
  if (to < from) return 0
  if (unit === 'calendar') return Math.round((to.getTime() - from.getTime()) / 86_400_000) + 1
  let counted = 0
  for (const d = from; d <= to && counted < MAX_DAYS; d.setUTCDate(d.getUTCDate() + 1)) {
    if (await isWorkday(d)) counted++
  }
  return counted
}

export function useWorkCalendar() {
  return { fallback, endFromDuration, durationFromDates }
}
