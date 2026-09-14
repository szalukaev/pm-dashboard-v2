import { createI18n } from 'vue-i18n'
import ru from './locales/ru.json'
import en from './locales/en.json'

const savedLang = localStorage.getItem('pm-dashboard-lang') || navigator.language.split('-')[0] || 'ru'

const i18n = createI18n({
  legacy: false,
  locale: savedLang,
  fallbackLocale: 'ru',
  messages: { ru, en }
})

export default i18n
